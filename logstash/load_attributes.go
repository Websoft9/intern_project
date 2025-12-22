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
	"strings"
	"time"
)

// ProductSeries represents the top level product series
type ProductSeries struct {
	XMLName       xml.Name      `xml:"productSeries"`
	ProductSeries []ProductBase `xml:"productSeries"`
}

// ProductBase represents a base product
type ProductBase struct {
	Products Products `xml:"products"`
}

// Products contains a list of products
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

// Attribute represents an attribute in our internal structure
type Attribute struct {
	Name         string   `json:"name"`
	FeatureType  string   `json:"featureType"`
	FeatureTerms []string `json:"featureTerms"`
}

// CurrentAttributeItem represents a single attribute item from the API
type CurrentAttributeItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Taxonomy     string   `json:"taxonomy"`
	FeatureTerms []string `json:"featureTerms"`
}

// CurrentAttributeStructure represents the API response for get-attributes
type CurrentAttributeStructure map[string]CurrentAttributeItem

var (
	apiHost = "https://recom-power.cn/wp-json/recom-wordpress/v1/"
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
	fmt.Println("Starting attribute synchronization...")

	// Initialize variables.
	catalogAttributeStructure := make(map[string]*Attribute)
	currentAttributeStructure := make(CurrentAttributeStructure)

	// Statistics counters
	stats := struct {
		NewAttributes     int
		ExistingAttributes int
		NewAttributeTerms int
		ExistingAttributeTerms int
	}{}

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

	// Build the attribute structure.
	for _, productBase := range catalog.ProductSeries {
		for _, productVariation := range productBase.Products.Product {
			features := productVariation.Features

			// Process numeric features
			for _, featureItem := range features.NumericFeature {
				if _, exists := catalogAttributeStructure[featureItem.ID]; !exists {
					catalogAttributeStructure[featureItem.ID] = &Attribute{
						Name:         maybeCutAttributeName(featureItem.ID),
						FeatureType:  "numericFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value) {
					catalogAttributeStructure[featureItem.ID].FeatureTerms = append(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value)
				}
			}

			// Process enum features
			for _, featureItem := range features.EnumFeature {
				if _, exists := catalogAttributeStructure[featureItem.ID]; !exists {
					catalogAttributeStructure[featureItem.ID] = &Attribute{
						Name:         maybeCutAttributeName(featureItem.ID),
						FeatureType:  "enumFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value) {
					catalogAttributeStructure[featureItem.ID].FeatureTerms = append(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value)
				}
			}

			// Process range features
			for _, featureItem := range features.RangeFeature {
				// Save as range in one field
				rangeKey := "range_" + featureItem.ID
				if _, exists := catalogAttributeStructure[rangeKey]; !exists {
					catalogAttributeStructure[rangeKey] = &Attribute{
						Name:         maybeCutAttributeName(rangeKey),
						FeatureType:  "rangeFeature",
						FeatureTerms: []string{},
					}
				}

				rangeValue := "[" + featureItem.From + ", " + featureItem.To + "]"
				if !contains(catalogAttributeStructure[rangeKey].FeatureTerms, rangeValue) {
					catalogAttributeStructure[rangeKey].FeatureTerms = append(catalogAttributeStructure[rangeKey].FeatureTerms, rangeValue)
				}

				// Save as one value in two (another) fields
				rangeFromKey := "rangefrom_" + featureItem.ID
				if _, exists := catalogAttributeStructure[rangeFromKey]; !exists {
					catalogAttributeStructure[rangeFromKey] = &Attribute{
						Name:         maybeCutAttributeName(rangeFromKey),
						FeatureType:  "rangeFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[rangeFromKey].FeatureTerms, featureItem.From) {
					catalogAttributeStructure[rangeFromKey].FeatureTerms = append(catalogAttributeStructure[rangeFromKey].FeatureTerms, featureItem.From)
				}

				rangeToKey := "rangeto_" + featureItem.ID
				if _, exists := catalogAttributeStructure[rangeToKey]; !exists {
					catalogAttributeStructure[rangeToKey] = &Attribute{
						Name:         maybeCutAttributeName(rangeToKey),
						FeatureType:  "rangeFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[rangeToKey].FeatureTerms, featureItem.To) {
					catalogAttributeStructure[rangeToKey].FeatureTerms = append(catalogAttributeStructure[rangeToKey].FeatureTerms, featureItem.To)
				}
			}

			// Process set features - 这里是关键修改部分
			for _, featureItem := range features.SetFeature {
				// 对于 setFeature，我们为每个独立的值创建一个属性项
				// 这符合原始JS脚本的逻辑
				attrKey := featureItem.ID
				if _, exists := catalogAttributeStructure[attrKey]; !exists {
					catalogAttributeStructure[attrKey] = &Attribute{
						Name:         maybeCutAttributeName(attrKey),
						FeatureType:  "setFeature",
						FeatureTerms: []string{},
					}
				}

				// 将每个值添加为独立的特性项
				for _, value := range featureItem.Value {
					if !contains(catalogAttributeStructure[attrKey].FeatureTerms, value) {
						catalogAttributeStructure[attrKey].FeatureTerms = append(catalogAttributeStructure[attrKey].FeatureTerms, value)
					}
				}
			}

			// Process boolean features
			for _, featureItem := range features.BooleanFeature {
				if _, exists := catalogAttributeStructure[featureItem.ID]; !exists {
					catalogAttributeStructure[featureItem.ID] = &Attribute{
						Name:         maybeCutAttributeName(featureItem.ID),
						FeatureType:  "booleanFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value) {
					catalogAttributeStructure[featureItem.ID].FeatureTerms = append(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value)
				}
			}

			// Process day features
			for _, featureItem := range features.DayFeature {
				if _, exists := catalogAttributeStructure[featureItem.ID]; !exists {
					catalogAttributeStructure[featureItem.ID] = &Attribute{
						Name:         maybeCutAttributeName(featureItem.ID),
						FeatureType:  "dayFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value) {
					catalogAttributeStructure[featureItem.ID].FeatureTerms = append(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value)
				}
			}

			// Process string features
			for _, featureItem := range features.StringFeature {
				if _, exists := catalogAttributeStructure[featureItem.ID]; !exists {
					catalogAttributeStructure[featureItem.ID] = &Attribute{
						Name:         maybeCutAttributeName(featureItem.ID),
						FeatureType:  "stringFeature",
						FeatureTerms: []string{},
					}
				}

				if !contains(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value) {
					catalogAttributeStructure[featureItem.ID].FeatureTerms = append(catalogAttributeStructure[featureItem.ID].FeatureTerms, featureItem.Value)
				}
			}
		}
	}

	// Get and build current attribute structure.
	err = getCurrentAttributes(currentAttributeStructure)
	if err != nil {
		fmt.Printf("Failed to get current attributes: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Synchronize attributes.
	for _, attribute := range catalogAttributeStructure {
		taxonomy := "pa_" + attribute.Name
		taxonomy = strings.ToLower(taxonomy)

		if _, exists := currentAttributeStructure[attribute.Name]; exists {
			fmt.Printf("Attribute already exists: %s\n", attribute.Name)
			stats.ExistingAttributes++
		} else {
			fmt.Printf("Add attribute: %s\n", attribute.Name)
			err = addAttribute(attribute)
			if err != nil {
				fmt.Printf("Warning: Failed to add attribute %s: %v\n", attribute.Name, err)
				// Continue with other attributes instead of stopping everything
				continue
			}
			stats.NewAttributes++
		}

		// Synchronize attribute terms.
		for _, featureTerm := range attribute.FeatureTerms {
			if attr, exists := currentAttributeStructure[attribute.Name]; exists {
				if contains(attr.FeatureTerms, featureTerm) {
					fmt.Printf("Attribute term already exists: %s\n", featureTerm)
					stats.ExistingAttributeTerms++
				} else {
					fmt.Printf("Add attribute term: %s\n", featureTerm)
					err = addAttributeTerm(featureTerm, taxonomy)
					if err != nil {
						fmt.Printf("Warning: Failed to add attribute term %s: %v\n", featureTerm, err)
						// Continue with other terms instead of stopping everything
						continue
					}
					stats.NewAttributeTerms++
				}
			} else {
				fmt.Printf("Add attribute term: %s\n", featureTerm)
				err = addAttributeTerm(featureTerm, taxonomy)
				if err != nil {
					fmt.Printf("Warning: Failed to add attribute term %s: %v\n", featureTerm, err)
					// Continue with other terms instead of stopping everything
					continue
				}
				stats.NewAttributeTerms++
			}
		}
	}

	// Write statistics to log file
	logMessage := fmt.Sprintf(
		"[%s] Attribute synchronization completed. New attributes: %d, Existing attributes: %d, New attribute terms: %d, Existing attribute terms: %d\n",
		time.Now().Format("2006-01-02 15:04:05"),
		stats.NewAttributes,
		stats.ExistingAttributes,
		stats.NewAttributeTerms,
		stats.ExistingAttributeTerms,
	)

	// Append to log file
	logFile, err := os.OpenFile("/usr/share/logstash/data/statistics.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to open log file: %v\n", err)
	} else {
		defer logFile.Close()
		if _, err := logFile.WriteString(logMessage); err != nil {
			fmt.Printf("Warning: Failed to write to log file: %v\n", err)
		}
	}

	// Also print stats to console
	fmt.Printf("Statistics: %s", logMessage)

	// Return success response.
	fmt.Println("Attribute synchronization completed.")
	return &Response{StatusCode: 200}, nil
}

func getCurrentAttributes(currentAttributeStructure CurrentAttributeStructure) error {
	fmt.Println("Fetching current attributes from API...")
	resp, err := http.Get(apiHost + "get-attributes")
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

	err = json.Unmarshal(body, &currentAttributeStructure)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	fmt.Println("Current attributes fetched successfully.")
	return nil
}

func addAttribute(attribute *Attribute) error {
	fmt.Printf("Adding attribute: %s\n", attribute.Name)
	
	url := apiHost + "add-attribute"
	payload := map[string]interface{}{
		"name": attribute.Name,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request with proper headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
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

	fmt.Printf("Attribute %s added successfully.\n", attribute.Name)
	return nil
}

func addAttributeTerm(term, taxonomy string) error {
	fmt.Printf("Adding attribute term: %s with taxonomy: %s\n", term, taxonomy)
	
	url := apiHost + "add-attribute-term"
	payload := map[string]interface{}{
		"term":     term,
		"taxonomy": taxonomy,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request with proper headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
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

	fmt.Printf("Attribute term %s added successfully.\n", term)
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