package gitlog

import "strings"

// Draw fills each commit's Graph with one row of a lane graph. Commits must
// be in topological order, children before parents. Every lane is two cells
// wide: the lane glyph and a connector cell.
//
//	●       commit on this lane
//	│       a lane passing by
//	─╯ ─╮   a lane joining the commit (its child forked here) or a merge
//	        parent starting a new lane
func Draw(commits []Commit) {
	var lanes []string // the commit each lane waits for; "" is free
	find := func(h string) int {
		for i, l := range lanes {
			if l == h {
				return i
			}
		}
		return -1
	}
	free := func() int {
		for i, l := range lanes {
			if l == "" {
				return i
			}
		}
		lanes = append(lanes, "")
		return len(lanes) - 1
	}
	for ci := range commits {
		c := &commits[ci]
		col := find(c.Hash)
		if col < 0 {
			col = free()
		}
		// Other lanes waiting for the same commit join it here.
		var joins []int
		for i, l := range lanes {
			if i != col && l == c.Hash {
				joins = append(joins, i)
				lanes[i] = ""
			}
		}
		var forks []int
		if len(c.Parents) == 0 {
			lanes[col] = ""
		} else {
			lanes[col] = c.Parents[0]
			for _, p := range c.Parents[1:] {
				if i := find(p); i >= 0 {
					forks = append(forks, i)
					continue
				}
				i := free()
				lanes[i] = p
				forks = append(forks, i)
			}
		}
		width := len(lanes)
		row := make([]rune, width*2)
		for i := range row {
			row[i] = ' '
		}
		for i := 0; i < width; i++ {
			if lanes[i] != "" && i != col {
				row[i*2] = '│'
			}
		}
		connect := func(j int, right, left rune) {
			lo, hi := col, j
			glyph := right
			if j < col {
				lo, hi = j, col
				glyph = left
			}
			for k := lo*2 + 1; k < hi*2; k++ {
				if row[k] == ' ' {
					row[k] = '─'
				}
			}
			row[j*2] = glyph
		}
		for _, j := range joins {
			connect(j, '╯', '╰')
		}
		for _, j := range forks {
			connect(j, '╮', '╭')
		}
		row[col*2] = '●'
		c.Graph = strings.TrimRight(string(row), " ")
		for len(lanes) > 0 && lanes[len(lanes)-1] == "" {
			lanes = lanes[:len(lanes)-1]
		}
	}
}
