package utils

import (
	"fmt"
	"testing"
)

// the benchmark colours cells itself: fatih/color emits nothing when the test
// output is not a terminal
func colored(str string) string {
	return "\x1b[32m" + str + "\x1b[0m"
}

// containerLikeRows builds rows shaped like the containers list: six cells,
// most of them coloured.
func containerLikeRows(count int) [][]string {
	rows := make([][]string, count)
	for i := range rows {
		rows[i] = []string{
			colored("running"),
			colored("(healthy)"),
			fmt.Sprintf("project-service-%d", i),
			colored(fmt.Sprintf("%d.%02d%%", i%100, i%97)),
			colored(fmt.Sprintf("%d->80/tcp, %d->443/tcp", 8000+i, 9000+i)),
			colored("registry.example.com/team/image:1.2.3"),
		}
	}
	return rows
}

func BenchmarkRenderTable(b *testing.B) {
	for _, count := range []int{40, 500} {
		rows := containerLikeRows(count)
		b.Run(fmt.Sprintf("rows=%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := RenderTable(rows); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
