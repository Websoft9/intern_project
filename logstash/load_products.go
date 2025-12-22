package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

// ProductSeries represents the top level product series
type ProductSeries struct {
	XMLName       xml.Name      `xml:"productSeries"`
	ProductSeries []ProductBase `xml:"productSeries"`
}

// ProductBase represents a base product
type ProductBase struct {
	Code         string       `xml:"code,attr"`
	LastChange   string       `xml:"lastChange,attr"`
	Description  string       `xml:"description"`
	ImageUrl     string       `xml:"imageUrl"`
	ThumbnailUrl string       `xml:"thumbnailUrl,omitempty"`
	BulletPoints BulletPoints `xml:"bulletPoints"`
	Nrnd         string       `xml:"nrnd,attr"`
	EndOfLife    string       `xml:"endOfLife,attr"`
	ComingSoon   string       `xml:"comingSoon,attr"`
	New          string       `xml:"new,attr"`
	Focus        string       `xml:"focus,attr"`
	Categories   Categories   `xml:"categories"`
	Products     Products     `xml:"products"`
}

// BulletPoints contains bullet points
type BulletPoints struct {
	BulletPoint []string `xml:"bulletPoint"`
}

// Categories contains category information
type Categories struct {
	Category []Category `xml:"category"`
}

// Category represents a single category
type Category struct {
	Code string `xml:"code,attr"`
}

// Products contains product variations
type Products struct {
	Product []Product `xml:"product"`
}

// Product represents a product variation
type Product struct {
	Features Features `xml:"features"`
}

// Features contains all types of features
type Features struct {
	NumericFeature []NumericFeature `xml:"numericFeature"`
	SetFeature     []SetFeature     `xml:"setFeature"`
	RangeFeature   []RangeFeature   `xml:"rangeFeature"`
	EnumFeature    []EnumFeature    `xml:"enumFeature"`
	BooleanFeature []BooleanFeature `xml:"booleanFeature"`
	DayFeature     []DayFeature     `xml:"dayFeature"`
	StringFeature  []StringFeature  `xml:"stringFeature"`
}

// NumericFeature represents numeric feature
type NumericFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// EnumFeature represents enum feature
type EnumFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// RangeFeature represents range feature
type RangeFeature struct {
	ID   string `xml:"id,attr"`
	From string `xml:"from,attr"`
	To   string `xml:"to,attr"`
}

// SetFeature represents set feature
type SetFeature struct {
	ID    string   `xml:"id,attr"`
	Value []string `xml:"value"`
}

// BooleanFeature represents boolean feature
type BooleanFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// DayFeature represents day feature
type DayFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// StringFeature represents string feature
type StringFeature struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

// CatalogProduct represents a product in our internal structure
type CatalogProduct struct {
	Code             string              `json:"code"`
	LastChange       int64               `json:"lastChange"`
	Description      string              `json:"description"`
	ImageUrl         string              `json:"imageUrl"`
	ThumbnailUrl     string              `json:"thumbnailUrl"`
	BulletPoints     []string            `json:"bulletPoints"`
	Nrnd             int                 `json:"nrnd"`
	EndOfLife        int                 `json:"endOfLife"`
	ProductComingSoon int                `json:"productComingSoon"`
	ProductNew       int                 `json:"productNew"`
	ProductFocus     int                 `json:"productFocus"`
	Categories       []string            `json:"categories"`
	Attributes       map[string][]string `json:"attributes"`
}

// CurrentProductItem represents a product from the API
type CurrentProductItem struct {
	ID         int    `json:"id"`
	Code       string `json:"code"`
	LastChange string `json:"lastChange"` // Changed to string to match API response
}

// CurrentProductStructure represents the API response for get-products
type CurrentProductStructure map[string]CurrentProductItem

var (
	apiHost = "https://recom-power.cn/wp-json/recom-wordpress/v1/"
	
	// 统计变量
	addedCount   int // 新增产品计数
	updatedCount int // 更新产品计数
)

func main() {
	response, err := handler()
	if err != nil {
		fmt.Printf("Error occurred: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Function completed with status code: %d\n", response.StatusCode)
}

type Response struct {
	StatusCode int `json:"statusCode"`
}

func handler() (*Response, error) {
	// 初始化统计变量
	addedCount = 0
	updatedCount = 0
	
	fmt.Println("Starting product synchronization...")

	// Initialize variables.
	catalogProductStructure := make(map[string]*CatalogProduct)
	currentProductStructure := make(CurrentProductStructure)

	// Load the catalog into the memory.
	catalogData, err := ioutil.ReadFile("/usr/share/logstash/data/recom-catalog.xml")
	if err != nil {
		fmt.Printf("Failed to read products.xml: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Parse the catalog.
	var catalog ProductSeries
	err = xml.Unmarshal(catalogData, &catalog)
	if err != nil {
		fmt.Printf("Failed to parse XML: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Build the product structure.
	for _, product := range catalog.ProductSeries {
		catalogProductStructure[product.Code] = addProductToCatalog(product)
	}

	// Get and build current product structure.
	err = getCurrentProducts(currentProductStructure)
	if err != nil {
		fmt.Printf("Failed to get current products: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Synchronize products.
	for _, product := range catalogProductStructure {
		var productID int

		if currentItem, exists := currentProductStructure[product.Code]; exists {
			fmt.Printf("Product already exists: %s\n", product.Code)
			productID = currentItem.ID

			// Convert string lastChange from API to int64 for comparison
			currentLastChange, err := strconv.ParseInt(currentItem.LastChange, 10, 64)
			if err != nil {
				fmt.Printf("Warning: Failed to parse lastChange for existing product %s: %v\n", product.Code, err)
				currentLastChange = 0
			}

			if currentLastChange != product.LastChange {
				fmt.Printf("Update product: %s\n", product.Code)
				err = updateProduct(productID, product)
				if err != nil {
					fmt.Printf("Warning: Failed to update product %s: %v\n", product.Code, err)
					continue
				}
				updatedCount++ // 增加更新计数
			}
		} else {
			fmt.Printf("Add product: %s\n", product.Code)
			err = addProduct(product)
			if err != nil {
				fmt.Printf("Warning: Failed to add product %s: %v\n", product.Code, err)
				continue
			}
			addedCount++ // 增加新增计数
		}
	}

	// Return success response.
	fmt.Println("Product synchronization completed.")
	
	// 将统计结果写入日志文件
	logMessage := fmt.Sprintf("[%s] Product synchronization completed. Added: %d products, Updated: %d products\n", 
		time.Now().Format("2006-01-02 15:04:05"), addedCount, updatedCount)
	
	err = writeLog(logMessage)
	if err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}
	
	return &Response{StatusCode: 200}, nil
}

func addProductToCatalog(product ProductBase) *CatalogProduct {
	code := product.Code
	
	// Parse lastChange timestamp
	lastChangeTime, err := time.Parse(time.RFC3339, product.LastChange)
	var lastChange int64
	if err != nil {
		fmt.Printf("Warning: Failed to parse lastChange for product %s: %v\n", code, err)
		lastChange = 0
	} else {
		lastChange = lastChangeTime.Unix()
	}
	
	description := product.Description
	imageUrl := product.ImageUrl
	thumbnailUrl := product.ThumbnailUrl
	bulletPoints := product.BulletPoints.BulletPoint

	// Parse boolean values
	nrnd := parseBoolToInt(product.Nrnd)
	endOfLife := parseBoolToInt(product.EndOfLife)
	productComingSoon := parseBoolToInt(product.ComingSoon)
	productNew := parseBoolToInt(product.New)
	productFocus := parseBoolToInt(product.Focus)

	// Build categories
	categories := []string{}
	for _, category := range product.Categories.Category {
		categories = append(categories, category.Code)
	}

	// Build attributes
	attributes := make(map[string][]string)

	for _, productVariation := range product.Products.Product {
		features := productVariation.Features

		// Process numeric features
		for _, featureItem := range features.NumericFeature {
			attrKey := maybeCutAttributeName(featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}
			if !contains(attributes[attrKey], featureItem.Value) {
				attributes[attrKey] = append(attributes[attrKey], featureItem.Value)
			}
		}

		// Process enum features
		for _, featureItem := range features.EnumFeature {
			attrKey := maybeCutAttributeName(featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}
			if !contains(attributes[attrKey], featureItem.Value) {
				attributes[attrKey] = append(attributes[attrKey], featureItem.Value)
			}
		}

		// Process range features
		for _, featureItem := range features.RangeFeature {
			// Save as range in one field
			rangeKey := maybeCutAttributeName("range_" + featureItem.ID)
			if _, exists := attributes[rangeKey]; !exists {
				attributes[rangeKey] = []string{}
			}
			rangeValue := "[" + featureItem.From + ", " + featureItem.To + "]"
			if !contains(attributes[rangeKey], rangeValue) {
				attributes[rangeKey] = append(attributes[rangeKey], rangeValue)
			}

			// Save as one value in two (another) fields
			rangeFromKey := maybeCutAttributeName("rangefrom_" + featureItem.ID)
			if _, exists := attributes[rangeFromKey]; !exists {
				attributes[rangeFromKey] = []string{}
			}
			if !contains(attributes[rangeFromKey], featureItem.From) {
				attributes[rangeFromKey] = append(attributes[rangeFromKey], featureItem.From)
			}

			rangeToKey := maybeCutAttributeName("rangeto_" + featureItem.ID)
			if _, exists := attributes[rangeToKey]; !exists {
				attributes[rangeToKey] = []string{}
			}
			if !contains(attributes[rangeToKey], featureItem.To) {
				attributes[rangeToKey] = append(attributes[rangeToKey], featureItem.To)
			}
		}

		// Process set features
		for _, featureItem := range features.SetFeature {
			attrKey := maybeCutAttributeName("set_" + featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}

			// Sort the values
			sortedValues := make([]string, len(featureItem.Value))
			copy(sortedValues, featureItem.Value)
			sort.Strings(sortedValues)

			// Convert to JSON string
			valuesJSON, _ := json.Marshal(sortedValues)
			valuesStr := string(valuesJSON)

			if !contains(attributes[attrKey], valuesStr) {
				attributes[attrKey] = append(attributes[attrKey], valuesStr)
			}
		}

		// Process boolean features
		for _, featureItem := range features.BooleanFeature {
			attrKey := maybeCutAttributeName(featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}
			if !contains(attributes[attrKey], featureItem.Value) {
				attributes[attrKey] = append(attributes[attrKey], featureItem.Value)
			}
		}

		// Process day features
		for _, featureItem := range features.DayFeature {
			attrKey := maybeCutAttributeName(featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}
			if !contains(attributes[attrKey], featureItem.Value) {
				attributes[attrKey] = append(attributes[attrKey], featureItem.Value)
			}
		}

		// Process string features
		for _, featureItem := range features.StringFeature {
			attrKey := maybeCutAttributeName(featureItem.ID)
			if _, exists := attributes[attrKey]; !exists {
				attributes[attrKey] = []string{}
			}
			if !contains(attributes[attrKey], featureItem.Value) {
				attributes[attrKey] = append(attributes[attrKey], featureItem.Value)
			}
		}
	}

	return &CatalogProduct{
		Code:             code,
		LastChange:       lastChange,
		Description:      description,
		ImageUrl:         imageUrl,
		ThumbnailUrl:     thumbnailUrl,
		BulletPoints:     bulletPoints,
		Nrnd:             nrnd,
		EndOfLife:        endOfLife,
		ProductComingSoon: productComingSoon,
		ProductNew:       productNew,
		ProductFocus:     productFocus,
		Categories:       categories,
		Attributes:       attributes,
	}
}

func getCurrentProducts(currentProductStructure CurrentProductStructure) error {
	fmt.Println("Fetching current products from API...")
	resp, err := http.Get(apiHost + "get-products")
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Check if response is an empty array
	if string(body) == "[]" {
		fmt.Println("No existing products found.")
		return nil
	}

	err = json.Unmarshal(body, &currentProductStructure)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	fmt.Println("Current products fetched successfully.")
	return nil
}

func addProduct(product *CatalogProduct) error {
	fmt.Printf("Adding product: %s\n", product.Code)

	url := apiHost + "add-product"
	payload, err := json.Marshal(map[string]interface{}{
		"code":             product.Code,
		"lastChange":       product.LastChange,
		"description":      product.Description,
		"imageUrl":         product.ImageUrl,
		"thumbnailUrl":     product.ThumbnailUrl,
		"bulletPoints":     product.BulletPoints,
		"nrnd":             product.Nrnd,
		"endOfLife":        product.EndOfLife,
		"productComingSoon": product.ProductComingSoon,
		"productNew":       product.ProductNew,
		"productFocus":     product.ProductFocus,
		"categories":       product.Categories,
		"attributes":       product.Attributes,
	})

	if err != nil {
		return err
	}

	// Create request with proper headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Product %s added successfully.\n", product.Code)
	return nil
}

func updateProduct(productID int, product *CatalogProduct) error {
	fmt.Printf("Updating product ID: %d (%s)\n", productID, product.Code)

	url := apiHost + "update-product"
	payload, err := json.Marshal(map[string]interface{}{
		"id":               productID,
		"lastChange":       product.LastChange,
		"description":      product.Description,
		"imageUrl":         product.ImageUrl,
		"thumbnailUrl":     product.ThumbnailUrl,
		"bulletPoints":     product.BulletPoints,
		"nrnd":             product.Nrnd,
		"endOfLife":        product.EndOfLife,
		"productComingSoon": product.ProductComingSoon,
		"productNew":       product.ProductNew,
		"productFocus":     product.ProductFocus,
		"categories":       product.Categories,
		"attributes":       product.Attributes,
	})

	if err != nil {
		return err
	}

	// Create request with proper headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d, response: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Product ID %d updated successfully.\n", productID)
	return nil
}

func maybeCutAttributeName(name string) string {
	if len(name) > 28 {
		return name[:28]
	}
	return name
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func parseBoolToInt(value string) int {
	if value == "true" {
		return 1
	} else if value == "false" {
		return 0
	}
	
	// Try to parse as integer
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	
	return 0
}

// writeLog 将日志信息写入指定文件
func writeLog(message string) error {
	file, err := os.OpenFile("/usr/share/logstash/data/statistics.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(message)
	return err
}