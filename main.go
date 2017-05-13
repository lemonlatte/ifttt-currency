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

var lastPrice = 0.0
var cookies []*http.Cookie = []*http.Cookie{}

func requestETHPrice(retry int) (map[string]interface{}, error) {
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
		return nil, fmt.Errorf("maicoin request error: %s", err.Error())
	}
	defer resp.Body.Close()

	d := json.NewDecoder(resp.Body)
	v := map[string]interface{}{}

	err = d.Decode(&v)
	if err != nil {
		cookies = resp.Cookies()
		time.Sleep(time.Second)
		log.Print("parse error. sleep for 1 second.")
		return requestETHPrice(retry - 1)
	}
	return v, nil
}

func pushIFTTTEvent(price float64) error {
	log.Printf("current price: %0.4f", price)
	log.Printf("last price: %0.4f", lastPrice)
	if lastPrice == 0 {
		lastPrice = price
	}

	priceDiff := price / lastPrice
	priceRatio := price / lastPrice
	if math.Abs(priceRatio) > 5.0 || math.Abs(priceDiff) > 5.0 {
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
	}
	lastPrice = price
	return nil
}

func main() {
	for {
		v, err := requestETHPrice(3)
		if err != nil {
			log.Print(err)
		} else {
			if rawPrice, ok := v["raw_price_in_twd"]; !ok {
				log.Print("missing field")
			} else {
				price := rawPrice.(float64) / 100000

				if err := pushIFTTTEvent(price); err != nil {
					log.Printf("ifttt error: %s", err.Error())
				}
			}
		}
		time.Sleep(time.Minute)
	}
}
