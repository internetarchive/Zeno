package utils

import (
	"strconv"

	"github.com/zeebo/xxh3"
)

func GenSeencheckHash(URL string) string {
	return strconv.FormatUint(xxh3.HashString(URL), 10)
}
