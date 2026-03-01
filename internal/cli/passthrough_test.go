package cli

import "testing"

func TestShouldCleanup(t *testing.T) {
	tests := []struct {
		name   string
		subcmd string
		args   []string
		want   bool
	}{
		{
			name:   "down コマンドはクリーンアップする",
			subcmd: "down",
			args:   []string{"down"},
			want:   true,
		},
		{
			name:   "stop コマンドはクリーンアップする",
			subcmd: "stop",
			args:   []string{"stop"},
			want:   true,
		},
		{
			name:   "フォアグラウンド up はクリーンアップする",
			subcmd: "up",
			args:   []string{"up"},
			want:   true,
		},
		{
			name:   "デタッチモード up はクリーンアップしない",
			subcmd: "up",
			args:   []string{"up", "-d"},
			want:   false,
		},
		{
			name:   "ps コマンドはクリーンアップしない",
			subcmd: "ps",
			args:   []string{"ps"},
			want:   false,
		},
		{
			name:   "logs コマンドはクリーンアップしない",
			subcmd: "logs",
			args:   []string{"logs"},
			want:   false,
		},
		{
			name:   "restart コマンドはクリーンアップしない",
			subcmd: "restart",
			args:   []string{"restart"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCleanup(tt.subcmd, tt.args)
			if got != tt.want {
				t.Errorf("shouldCleanup(%q, %v) = %v, want %v", tt.subcmd, tt.args, got, tt.want)
			}
		})
	}
}
