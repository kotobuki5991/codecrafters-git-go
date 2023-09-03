package date

import (
	"fmt"
	"time"
)

const (
	COMMIT_DATE_FORMAT = "Mon Jan 2 15:04:05 2006 -0700"
)

func GetCommitDate() string {
	now := time.Now()
	commitDate := now.Format(COMMIT_DATE_FORMAT)
	return commitDate
}

func FormatNowTimezoneOffset() string {
	// 現在の日付と時刻を取得
	currentTime := time.Now()
	// Unixエポックからの経過秒数
	epochSeconds := currentTime.Unix()
	// タイムゾーンオフセット（秒数）
	offsetSeconds := currentTime.UTC().Sub(currentTime).Seconds()

	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}

	hours := int(offsetSeconds / 3600)
	minutes := int((offsetSeconds - float64(hours)*3600) / 60)

	formattedTimezoneOffset := fmt.Sprintf("%s%02d%02d", sign, hours, minutes)
	// タイムゾーンオフセットを文字列に変換
	timezoneOffsetStr := fmt.Sprintf("%d %s", int(epochSeconds), formattedTimezoneOffset)

	return timezoneOffsetStr
}
