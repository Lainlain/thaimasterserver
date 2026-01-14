package fcm

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

var (
	fcmClient *messaging.Client
)

// InitFCM initializes Firebase Cloud Messaging
func InitFCM(serviceAccountPath string) error {
	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("error initializing firebase app: %v", err)
	}

	fcmClient, err = app.Messaging(context.Background())
	if err != nil {
		return fmt.Errorf("error getting messaging client: %v", err)
	}

	log.Println("✅ Firebase Cloud Messaging initialized")
	return nil
}

// SendNotificationToTopic sends a notification to all devices subscribed to a topic
func SendNotificationToTopic(topic, title, body string) error {
	if fcmClient == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Title:        title,
				Body:         body,
				Sound:        "default",
				Priority:     messaging.PriorityMax,
				ChannelID:    "gift_notifications",
				Visibility:   messaging.VisibilityPublic,
				DefaultSound: true,
				Tag:          "gift_update",
			},
		},
		Topic: topic,
	}

	response, err := fcmClient.Send(context.Background(), message)
	if err != nil {
		log.Printf("❌ Error sending FCM notification: %v", err)
		return err
	}

	log.Printf("✅ FCM notification sent successfully: %s", response)
	return nil
}

// SendGiftAvailableNotification sends notification when a gift is updated
func SendGiftAvailableNotification(giftName string) error {
	title := giftName
	body := "Available 🎁"

	// Send to "gifts" topic - all users should subscribe to this topic
	return SendNotificationToTopic("gifts", title, body)
}

// SendNotificationWithImage sends a notification with optional image to a topic
func SendNotificationWithImage(topic, title, body, imageURL string) error {
	if fcmClient == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title:    title,
			Body:     body,
			ImageURL: imageURL,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Title:        title,
				Body:         body,
				Sound:        "default",
				Priority:     messaging.PriorityMax,
				ChannelID:    "general_notifications",
				Visibility:   messaging.VisibilityPublic,
				DefaultSound: true,
				ImageURL:     imageURL,
			},
		},
		Topic: topic,
	}

	response, err := fcmClient.Send(context.Background(), message)
	if err != nil {
		log.Printf("❌ Error sending FCM notification: %v", err)
		return err
	}

	log.Printf("✅ FCM notification sent successfully to topic '%s': %s", topic, response)
	return nil
}
