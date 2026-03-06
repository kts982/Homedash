package collector

import "io"

func closeQuietly(closer io.Closer) {
	_ = closer.Close()
}
