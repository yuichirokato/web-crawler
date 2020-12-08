package main

import (
	"log"

	"golang.org/x/net/context"

	firebase "firebase.google.com/go"

	"google.golang.org/api/option"
)

type FireStore struct {
	app    *App
	client *Firestore
}

func (f FireStore) Init() {
	ctx := context.Background()
	sa := option.WithCredentialsFile("/Users/yuichiroutakahashi/Documents/hobby/go/serviceAccount.json")

	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}

	f.app = app

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	f.client = client
}

func (f FireStore) Close() {
	f.client.Close()
}
