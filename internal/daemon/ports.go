package daemon

import (
	"fmt"
	"net"
)

// TryPorts returns the first TCP port in the range [start, start+count-1]
// that is free to bind on the loopback interface. It is the
// "port conflict retry" building block the spec asks for: if 7878 is
// already in use (because another instance is running, or some other
// tool grabbed it), we walk forward and pick the first port that's
// actually free.
//
// The probe is intentionally a bind+release: we open a listener on
// 127.0.0.1:port, see whether it errors with EADDRINUSE, and close it
// again. We do NOT dial the port (dialing can hit services that bind
// to other interfaces and would give a false negative).
//
// Returns the picked port, or an error if no port in the range is free.
func TryPorts(start, count int) (int, error) {
	if start <= 0 || start > 65535 {
		return 0, fmt.Errorf("invalid start port: %d", start)
	}
	if count <= 0 {
		count = 1
	}
	for i := 0; i < count; i++ {
		p := start + i
		if p > 65535 {
			break
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range [%d, %d)", start, start+count)
}
