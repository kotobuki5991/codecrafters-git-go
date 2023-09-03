package date

import (
	"fmt"
	"time"
)

const (
	COMMIT_DATE_FORMAT = "Mon Jan 2 15:04:05 2006 -0700"
)

func GetCommitDate()  {
	now := time.Now()
	commitDate := now.Format(COMMIT_DATE_FORMAT)
	fmt.Println(commitDate)
}
