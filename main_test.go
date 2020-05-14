package main

import "testing"

func TestFileHashes_AddDuplicates(t *testing.T) {

	type args struct {
		h [20]byte
		p string
	}
	tests := []struct {
		name     string
		existing string
		toAdd    string
		want     int
	}{
		{
			name:     "simple_add",
			existing: "",
			toAdd:    "aaaaaaaaaaaaaaaaaaaa",
			want:     0,
		},
		{
			name:     "dupe_add",
			existing: "aaaaaaaaaaaaaaaaaaaa",
			toAdd:    "aaaaaaaaaaaaaaaaaaaa",
			want:     1,
		},

		{
			name:     "nondupe_add",
			existing: "aaaaaaaaaaaaaaaaaaaa",
			toAdd:    "bbbbbbbbbbbbbbbbbbbb",
			want:     0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := make(map[[20]byte][]string)
			if tt.existing != "" {
				var b [20]byte
				copy(b[:], tt.existing)
				f[b] = []string{tt.existing}
			}

			var x [20]byte
			copy(x[:], tt.toAdd)
			f[x] = append(f[x], tt.toAdd)
			dups := duplicatesSHA1(f)
			if tt.want != len(dups) {
				t.Errorf("Duplicates() size = %v, want %v", len(dups), tt.want)
				t.Errorf("%+v\n", f)
				t.Errorf("%+v\n", dups)
			}
		})
	}
}
