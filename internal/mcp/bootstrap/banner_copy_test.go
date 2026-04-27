package bootstrap

import "testing"

func TestBannerCopy_Templates(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "installed_default_port",
			got:  BannerInstalled(7777),
			want: "Squad dashboard ready at http://localhost:7777",
		},
		{
			name: "installed_custom_port",
			got:  BannerInstalled(8080),
			want: "Squad dashboard ready at http://localhost:8080",
		},
		{
			name: "upgraded",
			got:  BannerUpgraded("0.3.0"),
			want: "Squad upgraded to 0.3.0; dashboard restarted",
		},
		{
			name: "port_conflict",
			got:  BannerPortConflict(7777),
			want: "Squad dashboard unavailable: port 7777 in use",
		},
		{
			name: "unsupported",
			got:  BannerUnsupported,
			want: `Squad dashboard auto-install not supported on this platform; run "squad serve" manually`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %q\nwant %q", tc.got, tc.want)
			}
		})
	}
}
