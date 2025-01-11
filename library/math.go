package library

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var tenToAny = map[int]string{0: "0", 1: "1", 2: "2", 3: "3", 4: "4", 5: "5", 6: "6", 7: "7", 8: "8", 9: "9", 10: "a", 11: "b", 12: "c", 13: "d", 14: "e", 15: "f", 16: "g", 17: "h", 18: "i", 19: "j", 20: "k", 21: "l", 22: "m", 23: "n", 24: "o", 25: "p", 26: "q", 27: "r", 28: "s", 29: "t", 30: "u", 31: "v", 32: "w", 33: "x", 34: "y", 35: "z", 37: "A", 38: "B", 39: "C", 40: "D", 41: "E", 42: "F", 43: "G", 44: "H", 45: "I", 46: "J", 47: "K", 48: "L", 49: "M", 50: "N", 51: "O", 52: "P", 53: "Q", 54: "R", 55: "S", 56: "T", 57: "U", 58: "V", 59: "W", 60: "X", 61: "Y", 62: "Z", 63: ":", 64: ";", 65: "<", 66: "=", 67: ">", 68: "?", 69: "@", 70: "[", 71: "]", 72: "^", 73: "_", 74: "{", 75: "|", 76: "}"}
var Letters = map[int]string{0: "a", 1: "b", 2: "c", 3: "d", 4: "e", 5: "f", 6: "g", 7: "h", 8: "i", 9: "j", 10: "k", 11: "l", 12: "m", 13: "n", 14: "o", 15: "p", 16: "q", 17: "r", 18: "s", 19: "t", 20: "u", 21: "v", 22: "w", 23: "x", 24: "y", 25: "z"}

func DecimalToAny(num int64, n int) string {
	newNumStr := ""
	var remainder int
	var remainderString string
	for num != 0 {
		remainder = int(num % int64(n))
		if 76 > remainder && remainder > 9 {
			remainderString = tenToAny[remainder]
		} else {
			remainderString = strconv.Itoa(remainder)
		}
		newNumStr = remainderString + newNumStr
		num = num / int64(n)
	}
	return newNumStr
}

func DecimalToLetter(num int64) string {
	newNumStr := ""
	for num != 0 {
		newNumStr = Letters[int(num%26)] + newNumStr
		num = num / int64(26)
	}
	return newNumStr
}

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

// GenerateRandString 生成随机字符串
func GenerateRandString(length int) string {
	const charSet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var output strings.Builder
	output.Grow(length) // 提前分配足够的空间

	// 初始化随机数生成器
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < length; i++ {
		// 随机选择一个字符
		character := charSet[rd.Intn(len(charSet))]
		output.WriteByte(character)
	}

	return output.String()
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

func Md5File(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	// 获取md5
	md5hash := md5.New()
	_, err = io.Copy(md5hash, file)
	if err != nil {
		return "", err
	}
	md5Str := hex.EncodeToString(md5hash.Sum(nil))

	return md5Str, nil
}

// VersionCompare 对比两个版本，a > b = 1; a < b = -1;
func VersionCompare(a string, b string) int {
	aSplit := strings.Split(a, ".")
	bSplit := strings.Split(b, ".")

	aLen := len(aSplit)
	bLen := len(bSplit)
	length := min(aLen, bLen)

	for i := 0; i < length; i++ {
		intA, _ := strconv.ParseInt(aSplit[i], 10, 32)
		intB, _ := strconv.ParseInt(bSplit[i], 10, 32)
		if intA > intB {
			return 1
		} else if intB > intA {
			return -1
		}
	}
	if aLen > bLen {
		return 1
	} else if bLen > aLen {
		return -1
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func LevenshteinDistance(s1, s2 string) float64 {
	r1 := []rune(s1)
	r2 := []rune(s2)
	len1 := len(r1)
	len2 := len(r2)

	prev := make([]float64, len2+1)
	curr := make([]float64, len2+1)

	for j := 0; j <= len2; j++ {
		prev[j] = float64(j)
	}

	for i := 1; i <= len1; i++ {
		curr[0] = float64(i)
		for j := 1; j <= len2; j++ {
			var cost float64
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			// 计算当前值
			curr[j] = math.Min(math.Min(prev[j-1]+cost, curr[j-1]+1), prev[j]+1)
		}
		// 滚动数组更新
		prev, curr = curr, prev
	}

	// 计算相似度
	distance := prev[len2]
	maxLen := math.Max(float64(len1), float64(len2))
	return 1 - distance/maxLen
}
