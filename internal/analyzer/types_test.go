package analyzer

import "testing"

func TestIdentical(t *testing.T) {
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"sameSlice", &SliceType{IntT}, &SliceType{IntT}, true},
		{"diffSlice", &SliceType{IntT}, &SliceType{StrT}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := identical(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("identical(%s, %s) = %v, want %v",
					tc.a.String(), tc.b.String(), got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		name string
		nt   Type
		want string
	}{
		{"FunctionType", &FunctionType{Params: []Param{{"a", IntT}},
			ReturnType: &ResultType{StrT}}, "fn(int) -> str!",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.nt.String()
			if got != tc.want {
				t.Errorf("%s != %s", got, tc.want)
			}
		})
	}
}
