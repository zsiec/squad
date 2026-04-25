package notify

import "time"

func defaultNow() int64 { return time.Now().Unix() }
