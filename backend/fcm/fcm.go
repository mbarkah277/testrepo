package fcm

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

var app *firebase.App

// InitFirebase loads the service account key and initializes the FCM client
func InitFirebase() {
	opt := option.WithCredentialsFile("firebase-key.json")
	var err error
	app, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Printf("[FCM] Warning: Firebase tidak dapat divalidasi (Cek firebase-key.json): %v\n", err)
	} else {
		log.Println("[FCM] Mesin Peluncur Misil Firebase Aktif bersandar di firebase-key.json!")
	}
}

// SendWakeUpSignal fires a High-Priority stealth push notification to Android
func SendWakeUpSignal(fcmToken string, command string) error {
	if app == nil || fcmToken == "" {
		return nil
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return err
	}

	message := &messaging.Message{
		Data: map[string]string{
			"action":  "wakeup",
			"command": command,
		},
		Token: fcmToken,
		Android: &messaging.AndroidConfig{
			Priority: "high", // This is crucial to penetrate Android's Deep Sleep (Doze Mode)
		},
	}

	response, err := client.Send(context.Background(), message)
	if err != nil {
		log.Printf("[FCM] Meleset! Gagal menembak roket ke HP target: %v\n", err)
		return err
	}

	log.Printf("[FCM] Tembakan sukses! OS Android dipaksa bangun. ID Roket: %s\n", response)
	return nil
}
