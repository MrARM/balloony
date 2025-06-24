package main

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("[*] Connected to MQTT")
	Subscribe(client, "batch")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("[!] Connect lost: %v\r\n", err)
}

func Subscribe(client mqtt.Client, topic string) {
	token := client.Subscribe(topic, 1, nil)
	if token.Wait() && token.Error() != nil {
		fmt.Println("[!] Error subscribing to topic:", token.Error())
	} else {
		fmt.Printf("[*] Subscribed to topic: %s\r\n", topic)
	}
}

func MQTTConnection(broker string, port int, clientID string, messageHandler mqtt.MessageHandler) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("wss://%s:%d", broker, port))
	opts.SetClientID(clientID)
	opts.SetDefaultPublishHandler(messageHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	opts.AutoReconnect = true
	return mqtt.NewClient(opts)
}
