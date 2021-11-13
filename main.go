package main

import (
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/net/html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

var errorLogger = log.New(os.Stderr, "", log.Llongfile)

func main() {
	lambda.Start(checkPayment)
}

type response struct {
	PriceZero bool `json:"price_zero"`
}

func checkPayment(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	client := &http.Client{}
	orderNumber := req.QueryStringParameters[orderNumberParamName]
	albumOrderNumber := req.QueryStringParameters[albumOrderNumberParamName]
	sessionId := req.Headers[sessionIdHeaderName]
	log.Printf("checking payment for %s/%s", orderNumber, albumOrderNumber)

	if len(orderNumber) > 0 && len(albumOrderNumber) > 0 && len(sessionId) > 0 {
		request := prepareRequest(orderNumber, sessionId)

		resp, _ := client.Do(request)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		parsed, _ := html.Parse(strings.NewReader(string(b)))
		id := getElementById(parsed, albumOrderNumber)
		if id != nil {
			data := id.LastChild.PrevSibling.FirstChild.Data
			js, err := json.Marshal(response{PriceZero: data == priceZeroHtmlColumnValue})
			if err != nil {
				return serverError(err)
			}
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusOK,
				Body:       string(js),
			}, nil
		}
	}
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNoContent,
	}, nil
}

func prepareRequest(orderNumber string, sessionId string) *http.Request {
	request, err := http.NewRequest(http.MethodGet, panelOrderDetailsUrl+orderNumber, nil)
	if err != nil {
		return nil
	}
	request.Header.Add(cookieHeaderName, panelSessionIdKey+sessionId)
	return request
}

func serverError(err error) (events.APIGatewayProxyResponse, error) {
	errorLogger.Println(err.Error())

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       http.StatusText(http.StatusInternalServerError),
	}, nil
}

func getAttribute(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

func checkId(n *html.Node, id string) bool {
	if n.Type == html.ElementNode {
		s, ok := getAttribute(n, albumOrderNumberColumnId)
		if ok && s == id {
			return true
		}
	}
	return false
}

func traverse(n *html.Node, id string) *html.Node {
	if checkId(n, id) {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result := traverse(c, id)
		if result != nil {
			return result
		}
	}

	return nil
}

func getElementById(n *html.Node, id string) *html.Node {
	return traverse(n, id)
}
