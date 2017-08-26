package iox

import "io"

type sectionWriter struct {
	w   io.WriterAt
	off int64
}

func NewSectionWriter(r io.WriterAt, off int64) io.Writer {
	return &sectionWriter{r, off}
}

func (s *sectionWriter) Write(p []byte) (n int, err error) {
	n, err = s.w.WriteAt(p, s.off)
	s.off += int64(n)
	return
}
