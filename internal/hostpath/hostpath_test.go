package hostpath

import (
	"strings"
	"testing"
)

const (
	wsl2Mountinfo = `21 26 0:20 / /sys rw,nosuid - sysfs sysfs rw
79 82 0:36 / /usr/lib/wsl/drivers ro,noatime - 9p drivers ro,aname=drivers;cache=0x5
131 82 0:69 / /mnt/c rw,noatime - 9p C:\134 rw,aname=drvfs;path=C:\;uid=1000;symlinkroot=/mnt/
132 82 0:70 / /mnt/my\040disk rw,noatime - 9p D:\134 rw,aname=drvfs;path=D:\;uid=1000
`
	wsl1Mountinfo = `24 20 0:23 / /mnt/c rw,noatime - drvfs C: rw,uid=1000
`
	linuxMountinfo = `21 26 0:20 / /sys rw,nosuid - sysfs sysfs rw
26 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw
`
)

func newTest(t *testing.T, mountinfo, distro string, tempDirs []string, opts Options) *Resolver {
	t.Helper()
	return New(Env{
		Lookup:    func(string) (string, bool) { return distro, distro != "" },
		Mountinfo: strings.NewReader(mountinfo),
		TempDirs:  tempDirs,
	}, opts)
}

func TestParseDrives(t *testing.T) {
	tests := []struct {
		name      string
		mountinfo string
		want      map[string]string
	}{
		{"wsl2", wsl2Mountinfo, map[string]string{"c": "/mnt/c", "d": "/mnt/my disk"}},
		{"wsl1", wsl1Mountinfo, map[string]string{"c": "/mnt/c"}},
		{"plain linux", linuxMountinfo, nil},
		{"truncated line", "24 20 0:23 / /mnt/c rw,noatime - drvfs C:\n", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDrives(strings.NewReader(tt.mountinfo))
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("drive %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}

	t.Run("nil reader", func(t *testing.T) {
		if got := parseDrives(nil); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})
}

func TestIsWindows(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{`C:\Users\me\file.php`, true},
		{"C:/Users/me/file.php", true},
		{"c:/x", true},
		{`\\wsl.localhost\Ubuntu\home`, true},
		{`\\server\share\file`, true},
		{"//wsl.localhost/Ubuntu/home", true},
		{"//wsl$/Ubuntu/home", true},
		{"//WSL.LOCALHOST/Ubuntu/home", true},
		// Only a distribution host tells a forward slash UNC path from an ordinary one.
		{"//server/share/file", false},
		{"//home/me/file.php", false},
		{"//", false},
		{"/home/me/file.php", false},
		{"relative/file.php", false},
		{"C:", false},
		{"C:file", false},
		{"1:/x", false},
		// The argument that started this: a regular expression full of backslashes.
		{`/(Squrious\\GlanceWidgetBundle\\Tests\\Functional\\RoutingTest::testIt)( .*)?$/`, false},
		{"--no-configuration", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWindows(tt.value); got != tt.want {
			t.Errorf("IsWindows(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestToLinux(t *testing.T) {
	r := newTest(t, wsl2Mountinfo, "Ubuntu-24.04", nil, Options{})
	tests := []struct {
		name  string
		value string
		want  string
		ok    bool
	}{
		{"drive backslashes", `C:\Users\Nicolas Rigaud\AppData\Local\Temp\ide-phpunit.php`, "/mnt/c/Users/Nicolas Rigaud/AppData/Local/Temp/ide-phpunit.php", true},
		{"drive forward slashes", "C:/Users/Nicolas Rigaud/AppData/Local/Temp/ide-phpunit.php", "/mnt/c/Users/Nicolas Rigaud/AppData/Local/Temp/ide-phpunit.php", true},
		{"lowercase drive", `c:\Windows\Temp\x`, "/mnt/c/Windows/Temp/x", true},
		{"escaped mount point", `D:\data\x.txt`, "/mnt/my disk/data/x.txt", true},
		{"drive root", `C:\`, "/mnt/c", true},
		{"unmounted drive", `Z:\foo\bar`, "", false},
		{"own distro", `\\wsl.localhost\Ubuntu-24.04\home\squrious\dev\lib\tests`, "/home/squrious/dev/lib/tests", true},
		{"own distro, forward slashes", "//wsl.localhost/Ubuntu-24.04/home/squrious/dev/lib/vendor/bin/phpunit", "/home/squrious/dev/lib/vendor/bin/phpunit", true},
		{"wsl$ alias, forward slashes", "//wsl$/Ubuntu-24.04/home/me", "/home/me", true},
		{"other distro, forward slashes", "//wsl.localhost/Debian/home/me", "", false},
		{"ordinary path with a doubled slash", "//home/me/x", "", false},
		{"own distro, other case", `\\wsl.localhost\UBUNTU-24.04\home\me`, "/home/me", true},
		{"wsl$ alias", `\\wsl$\Ubuntu-24.04\home\me`, "/home/me", true},
		{"distro root", `\\wsl.localhost\Ubuntu-24.04`, "/", true},
		{"other distro", `\\wsl.localhost\Debian\home\me`, "", false},
		{"network share", `\\fileserver\share\x`, "", false},
		{"not a windows path", "/home/me/x", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := r.ToLinux(tt.value)
			if ok != tt.ok || got != tt.want {
				t.Errorf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}

	t.Run("outside wsl", func(t *testing.T) {
		off := newTest(t, linuxMountinfo, "", nil, Options{})
		for _, v := range []string{`C:\x`, `\\wsl.localhost\Ubuntu-24.04\home\me`} {
			if got, ok := off.ToLinux(v); ok {
				t.Errorf("ToLinux(%q) = %q, want no conversion", v, got)
			}
		}
	})
}

func TestAllowed(t *testing.T) {
	r := newTest(t, wsl2Mountinfo, "Ubuntu-24.04", []string{"/custom/tmp", "/tmp", "/var/tmp"}, Options{
		Allow: []string{"/srv/fixtures/", `E:\shared`, "relative/ignored"},
	})
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"TempDir", "/custom/tmp/x", true},
		{"/tmp", "/tmp/x", true},
		{"/var/tmp", "/var/tmp/sub/x", true},
		{"configured, trailing slash cleaned", "/srv/fixtures/data.json", true},
		{"configured root itself", "/srv/fixtures", true},
		{"near miss on a root", "/srv/fixturesX/data.json", false},
		{"windows user temp", "/mnt/c/Users/Nicolas Rigaud/AppData/Local/Temp/ide-phpunit.php", true},
		{"windows user temp, other case", "/mnt/c/users/me/appdata/local/temp/x", true},
		{"windows system temp", "/mnt/c/Windows/Temp/x", true},
		{"windows temp on another drive", "/mnt/my disk/Users/me/AppData/Local/Temp/x", true},
		{"not quite windows temp", "/mnt/c/Users/me/AppData/Local/Temporary/x", false},
		{"nested under users", "/mnt/c/Users/me/sub/AppData/Local/Temp/x", false},
		{"windows documents", "/mnt/c/Users/me/Documents/x.php", false},
		{"unmounted drive path shape", "/mnt/z/Windows/Temp/x", false},
		{"ordinary home file", "/home/me/project/x.php", false},
		{"root", "/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Allowed(tt.path, tt.path); got != tt.want {
				t.Errorf("Allowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// A symlink is described by two paths: where it sits, and what it points at.
func TestAllowedSymlinks(t *testing.T) {
	const (
		link    = "/srv/fixtures/link.txt"
		outside = "/opt/data/secret.txt"
		inside  = "/srv/fixtures/plain.txt"
	)
	for _, tt := range []struct {
		name   string
		follow bool
		want   bool
	}{
		{"off by default", false, false},
		{"follow_symlinks", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newTest(t, linuxMountinfo, "", nil, Options{
				Allow: []string{"/srv/fixtures"}, FollowSymlinks: tt.follow,
			})
			if got := r.Allowed(link, outside); got != tt.want {
				t.Errorf("link out of an allowed root: got %v, want %v", got, tt.want)
			}
			// Neither a plain file inside nor one fully outside is affected.
			if !r.Allowed(inside, inside) {
				t.Error("a plain file in an allowed root should be allowed")
			}
			if r.Allowed(outside, outside) {
				t.Error("a file outside every root should not be allowed")
			}
			// A link pointing into an allowed root is allowed whatever the setting.
			if !r.Allowed("/opt/data/link.txt", inside) {
				t.Error("a link into an allowed root should be allowed")
			}
		})
	}
}

func TestAllowedWindowsRoot(t *testing.T) {
	r := newTest(t, wsl2Mountinfo, "Ubuntu-24.04", nil, Options{Allow: []string{`C:\shared`}})
	if !r.Allowed("/mnt/c/shared/x.txt", "/mnt/c/shared/x.txt") {
		t.Error("a Windows allow entry should be converted to its Linux path")
	}
}
