package library

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func GenerateRandNumber(length uint) uint {
	numberByteArray := [9]byte{1, 2, 3, 4, 5, 6, 7, 9}
	numberLength := len(numberByteArray)
	rand.Seed(time.Now().UnixNano())

	var stringBuilder strings.Builder
	for i := 0; uint(i) < length; i++ {
		fmt.Fprintf(&stringBuilder, "%d", numberByteArray[rand.Intn(numberLength)])
	}
	randomNumber, _ := strconv.ParseUint(stringBuilder.String(), 10, 0)
	return uint(randomNumber)
}

func Md5(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func Md5Bytes(bytes []byte) string {
	h := md5.New()
	h.Write(bytes)
	return hex.EncodeToString(h.Sum(nil))
}
