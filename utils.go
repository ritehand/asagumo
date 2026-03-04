package asagumo

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func NormalizeNumber(input string) (int, bool) {
	// 全角数字を半角に変換
	input = strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, input)

	// 数字の塊を探す
	reNum := regexp.MustCompile(`[0-9]+`)
	if match := reNum.FindString(input); match != "" {
		val, _ := strconv.Atoi(match)
		return val, true
	}

	// 漢数字の塊を探す
	reKanji := regexp.MustCompile(`[一二三四五六七八九十]+`)
	if match := reKanji.FindString(input); match != "" {
		return parseKanjiNumber(match)
	}

	return 0, false
}

func parseKanjiNumber(s string) (int, bool) {
	kanjiMap := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9,
	}

	res := 0
	tmp := 0
	for _, r := range s {
		if val, ok := kanjiMap[r]; ok {
			tmp = val
		} else if r == '十' {
			if tmp == 0 {
				tmp = 1
			}
			res += tmp * 10
			tmp = 0
		} else {
			return 0, false
		}
	}
	res += tmp
	return res, res > 0
}

func RetryOnRateLimit(f func() error) error {
	for {
		err := f()
		if err == nil {
			return nil
		}

		restErr, ok := err.(*discordgo.RESTError)
		if !ok || restErr.Response == nil || restErr.Response.StatusCode != http.StatusTooManyRequests {
			return err
		}

		resp := restErr.Response
		retryAfterStr := resp.Header.Get("Retry-After")
		resetAfterStr := resp.Header.Get("X-RateLimit-Reset-After")

		retryAfter, _ := strconv.ParseFloat(retryAfterStr, 64)
		if retryAfter == 0 {
			retryAfter, _ = strconv.ParseFloat(resetAfterStr, 64)
		}

		if retryAfter == 0 {
			retryAfter = 5
		}

		waitSec := time.Duration(retryAfter * float64(time.Second))
		resetTime := time.Now().Add(waitSec)

		log.Printf("!!! RATE LIMITED !!! Wait %v (Reset at %v)", waitSec, resetTime.Format("15:04:05.000"))

		time.Sleep(waitSec + 500*time.Millisecond)
	}
}
