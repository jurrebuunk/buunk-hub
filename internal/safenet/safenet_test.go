package safenet

import "testing"

func TestPublicOnly(t *testing.T) {
	for _, a := range []string{"10.43.0.10:443", "192.168.1.1:443", "127.0.0.1:443", "169.254.169.254:443", "100.100.1.1:443", "[::1]:443", "[fd00::1]:443", "[::ffff:10.0.0.1]:443"} {
		if PublicOnly("tcp", a, nil) == nil {
			t.Errorf("%s allowed", a)
		}
	}
	for _, a := range []string{"104.16.1.1:443", "[2606:4700::1]:443"} {
		if err := PublicOnly("tcp", a, nil); err != nil {
			t.Errorf("%s refused: %v", a, err)
		}
	}
}
