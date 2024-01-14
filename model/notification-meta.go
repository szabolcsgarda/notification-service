package model

type NotificationMeta struct {
	Notification  Notification `json:"notification"`
	ReceiptHandle string       `json:"handle"`
	QueueUrl      string       `json:"queue_url"`
}

func CreateNotificationMeta(notification Notification, handle string, queueUrl string) NotificationMeta {
	return NotificationMeta{
		Notification:  notification,
		ReceiptHandle: handle,
		QueueUrl:      queueUrl,
	}
}
