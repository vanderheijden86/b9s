package main_test

import "testing"

// Rows as the tree renders them under each colour profile. The CI rows are
// copied from a GitHub Actions run, where the runner gets the 16-colour profile.
func TestTitleHasBackground(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  string
		want bool
	}{
		{
			name: "16-colour selected row",
			row:  "\x1b[94m▸\x1b[0m \x1b[95m♦\x1b[0m \x1b[92;42mOPEN\x1b[0m \x1b[30m   \x1b[0m \x1b[46m\x1b[1;30mEpic One   \x1b[0m",
			want: true,
		},
		{
			name: "16-colour unselected row with a coloured badge",
			row:  "\x1b[94m▸\x1b[0m \x1b[95m♦\x1b[0m \x1b[92;42mOPEN\x1b[0m \x1b[36m   \x1b[0m \x1b[37mEpic One   \x1b[0m",
			want: false,
		},
		{
			name: "256-colour selected row",
			row:  "\x1b[38;5;42mOPEN\x1b[0m \x1b[1;38;5;16;48;5;51mEpic One\x1b[0m",
			want: true,
		},
		{
			name: "256-colour foreground index that looks like a background code",
			row:  "\x1b[38;5;42mEpic One\x1b[0m",
			want: false,
		},
		{
			name: "truecolor selected row",
			row:  "\x1b[48;2;0;200;200m\x1b[38;2;0;0;0mEpic One\x1b[0m",
			want: true,
		},
		{
			name: "background reset before the title",
			row:  "\x1b[46mx\x1b[49m Epic One",
			want: false,
		},
		{
			name: "title absent",
			row:  "\x1b[46mEpic Two",
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := titleHasBackground(tc.row, "Epic One"); got != tc.want {
				t.Errorf("titleHasBackground = %t, want %t", got, tc.want)
			}
		})
	}
}
