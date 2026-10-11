package update

import "testing"

// Version comparison must not crash on anything a server could send, and
// two versions can never both be newer than each other.
func FuzzNewer(f *testing.F) {
	for _, s := range [][2]string{{"1.2.3", "1.3.0"}, {"v1.0.0", "1.0.0"}, {"dev", "9.9.9"}, {"1.2", "1.2.3.4"}, {"-1.0.0", "1.0.0"}} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		if Newer(a, b) && Newer(b, a) {
			t.Fatalf("%q and %q are each newer than the other", a, b)
		}
		if Newer(a, a) {
			t.Fatalf("%q is newer than itself", a)
		}
	})
}
