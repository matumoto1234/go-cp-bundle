package packageutil_test

import (
	"testing"

	"github.com/matumoto1234/gocpbundle/packageutil"
)

func TestIsStandardPackage(t *testing.T) {
	tests := []struct {
		name string
		pkg  string
		want bool
	}{
		{
			"正常系 : 標準パッケージfmtの場合、標準パッケージだと判定される",
			"fmt",
			true,
		},
		{
			"正常系 : 標準パッケージnet/httpの場合、標準パッケージだと判定される",
			"net/http",
			true,
		},
		{
			"異常系 : 存在しないパッケージの場合、標準パッケージだと判定されない",
			"hoge",
			false,
		},
		{
			"異常系 : expパッケージの場合、標準パッケージだと判定されない",
			"golang.org/x/exp/constraints",
			false,
		},
		{
			"異常系 : 空文字列の場合、標準パッケージだと判定されない",
			"",
			false,
		},
		{
			"異常系 : ドットインポートの場合、標準パッケージだと判定されない",
			".",
			false,
		},
		{
			"異常系 : ブランクインポートの場合、標準パッケージだと判定されない",
			"_",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packageutil.IsStandardPackageName(tt.pkg)
			if got != tt.want {
				t.Fatalf("expected: %v, actual: %v\n", tt.want, got)
			}
		})
	}
}
