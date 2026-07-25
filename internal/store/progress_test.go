package store

import "testing"

func TestProgressForExperience(t *testing.T) {
	tests := []struct {
		experience int64
		level      int
		name       string
		progress   int
	}{
		{experience: 0, level: 1, name: "新芽", progress: 0},
		{experience: 50, level: 1, name: "新芽", progress: 50},
		{experience: 100, level: 2, name: "熟面孔", progress: 0},
		{experience: 800, level: 4, name: "资深玩家", progress: 0},
		{experience: 5000, level: 6, name: "社区元老", progress: 100},
	}
	for _, test := range tests {
		got := progressForExperience(test.experience)
		if got.Level != test.level || got.LevelName != test.name || got.LevelProgress != test.progress {
			t.Fatalf("progressForExperience(%d) = level %d %q progress %d", test.experience, got.Level, got.LevelName, got.LevelProgress)
		}
	}
}
