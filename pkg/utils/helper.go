package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func Debug(obj any) {
	raw, _ := json.MarshalIndent(obj, "", "/t")
	fmt.Println(raw)
}

func LocalTime() time.Time {
	loc, _ := time.LoadLocation("Asia/Bangkok")
	return time.Now().In(loc)
}

func CovertStringTimeToTime(t string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
	}

	for _, layout := range layouts {
		result, err := time.Parse(layout, t)
		if err == nil {
			return result
		}
	}

	log.Printf("Error: Parse time failed: %s", t)
	return time.Time{}
}
