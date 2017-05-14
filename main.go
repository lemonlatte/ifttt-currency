package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

const IFTTT_TOKEN = ""
const EVENT_PUSH_TIMEOUT = 6 * time.Hour

var currentPrice = 0.0
var lastPrice = 0.0
var cookies []*http.Cookie = []*http.Cookie{}

func requestETHPrice(retry int) (float64, error) {
	if retry == 0 {
		return 0, fmt.Errorf("retry timeout")
	}

	req, _ := http.NewRequest("GET", "https://www.maicoin.com/api/prices/eth-usd/", nil)
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")
	req.Header.Add("accept", "*/*")
	req.Header.Add("host", "www.maicoin.com")
	req.Header.Add("cache-control", "no-cache")

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("maicoin request error: %s", err.Error())
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	v := map[string]interface{}{}

	err = d.Decode(&v)
	if err != nil {
		cookies = resp.Cookies()
		time.Sleep(time.Second)
		log.Printf("parse error. sleep for 1 second. error: %s", err.Error())
		return requestETHPrice(retry - 1)
	}

	rawPrice, okKey := v["raw_price_in_twd"]
	if !okKey {
		return 0, fmt.Errorf("missing field")
	}

	price, okVal := rawPrice.(float64)
	if !okVal {
		return 0, fmt.Errorf("incorrect price type")
	}
	price /= 100000
	return price, nil
}

func pushIFTTTEvent(price, lastPrice float64) error {
	priceRatio := price / lastPrice
	changeText := ""
	if priceRatio > 1 {
		changeText = "📈"
	} else {
		changeText = "📉"
	}
	postBody := map[string]string{
		"value1": changeText,
		"value2": fmt.Sprintf("%0.4f", price),
		"value3": fmt.Sprintf("%+0.2f", (priceRatio-1)*100),
	}

	buf := bytes.Buffer{}
	e := json.NewEncoder(&buf)
	err := e.Encode(postBody)
	if err != nil {
		return err
	}

	r, err := http.Post(fmt.Sprintf("https://maker.ifttt.com/trigger/ether/with/key/%s", IFTTT_TOKEN),
		"application/json", &buf)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("invalid status return of ifttt event call: %s", r.Status)
	}
	return nil
}

func main() {

	priceAlert := make(chan *struct{})

	go func(priceAlert chan<- *struct{}) {
		var err error
		for {
			currentPrice, err = requestETHPrice(3)
			log.Printf("Current price: %0.4f. Last price: %0.4f", currentPrice, lastPrice)
			if err != nil {
				log.Print(err)
			} else {
				priceDiff := currentPrice - lastPrice
				priceRatio := currentPrice / lastPrice

				if math.Abs(priceRatio) > 5.0 || math.Abs(priceDiff) > 150.0 {
					log.Print("The difference of two price exceed the threshold. Push a new event.")
					priceAlert <- &struct{}{}
				}
			}
			time.Sleep(time.Minute)
		}
	}(priceAlert)

	for {
		select {
		case <-time.After(EVENT_PUSH_TIMEOUT):
		case <-priceAlert:
		}
		if err := pushIFTTTEvent(currentPrice, lastPrice); err != nil {
			log.Printf("IFTTT error: %s", err.Error())
		}
		lastPrice = currentPrice
	}
}
