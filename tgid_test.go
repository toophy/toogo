package toogo

import (
	"testing"
)

func Test_tgid(t *testing.T) {
	new_aid := Tgid_make_Aid(1, 1)

	println(Tgid_is_Aid(new_aid))
}
