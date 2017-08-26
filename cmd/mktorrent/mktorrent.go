package main

import (
	"bytes"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/takeyourhatoff/bt/internal/bencode"
	"github.com/takeyourhatoff/bt/internal/multifile"
	"golang.org/x/sync/errgroup"
)

const defaultComment = "created by cbv0"

var (
	out           = flag.String("out", "", "output file name (default: $path.torrent)")
	announce      = flag.String("announce", "", "announce url")
	comment       = flag.String("comment", defaultComment, "comment")
	private       = flag.Bool("private", false, "private flag")
	lgPieceLength = flag.Uint("piece-length", 20, "lg(piece-length) (default: 1MB)")
	cpuprofile    = flag.String("cpuprofile", "", "write cpuprofile to file")
)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if flag.Arg(0) == "" {
		flag.Usage()
		os.Exit(1)
	}
	name := flag.Arg(0)
	if *out == "" {
		*out = name + ".torrent"
	}
	var i bencode.Metainfo
	i.Announce = *announce
	i.Info.PieceLength = 1 << *lgPieceLength
	i.Info.Name = filepath.Base(name)
	i.Info.Private = *private
	i.CreationDate = time.Now()
	i.Comment = *comment

	err := makeMetainfo(name, &i)
	if err != nil {
		log.Fatal(err)
	}
	err = writeMetainfo(*out, i)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("infohash: %x\n", i.Info.Infohash(sha1.New()))
}

func makeMetainfo(name string, i *bencode.Metainfo) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	if info.IsDir() {
		err = filepath.Walk(name, func(name0 string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relname, err := filepath.Rel(name, name0)
			if err != nil {
				return err
			}
			i.Info.Files = append(i.Info.Files, bencode.File{
				Length: info.Size(),
				Path:   filepath.SplitList(relname),
			})
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		i.Info.Length = info.Size()
	}

	f, err := multifile.Open(filepath.Dir(name), *i)
	if err != nil {
		return err
	}
	p, err := pieces(f, int64(i.Info.PieceLength))
	if err != nil {
		return err
	}
	i.Info.Pieces = bytes.Join(p, nil)
	return nil
}

type sizeReaderAt interface {
	io.ReaderAt
	Size() int64
}

func pieces(r sizeReaderAt, len int64) ([][]byte, error) {
	var g errgroup.Group
	numPieces := int((r.Size() + len - 1) / len)
	p := make([][]byte, numPieces)
	c := make(chan int)
	for i := 0; i < runtime.NumCPU(); i++ {
		g.Go(func() error {
			h := sha1.New()
			var buf []byte
			for i := range c {
				rr := io.NewSectionReader(r, int64(i)*len, len)
				_, err := io.Copy(h, rr)
				if err != nil {
					return err
				}
				p[i] = h.Sum(buf[:0])
				h.Reset()
			}
			return nil
		})
	}
	for i := 0; i < numPieces; i++ {
		c <- i
	}
	close(c)
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	return p, nil
}

func writeMetainfo(name string, i bencode.Metainfo) (err error) {
	f, err := os.Create(name)
	if err != nil {
		return
	}
	defer func() {
		if err0 := f.Close(); err == nil {
			err = err0
		}
	}()
	err = bencode.Encode(f, i)
	return
}
