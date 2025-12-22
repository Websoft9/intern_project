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

// XMLCategory represents the XML structure of a category
type XMLCategory struct {
	Code        string        `xml:"code,attr"`
	Name        []XMLName     `xml:"name"`
	SubCategory []XMLCategory `xml:"subCategory"`
}

// XMLName represents the name element with language attribute
type XMLName struct {
	Value    string `xml:",chardata"`
	Language string `xml:"language,attr"`
}

// XMLCatalog represents the root categories element
type XMLCatalog struct {
	XMLName      xml.Name      `xml:"categories"`
	RootCategory []XMLCategory `xml:"rootCategory"`
}

// Category represents a category in our internal structure
type Category struct {
	Code     string              `json:"code"`
	Name     string              `json:"name"`
	Children map[string]*Category `json:"-"`
}

// CurrentCategoryChild represents a child category from the API
type CurrentCategoryChild struct {
	ID       int                             `json:"id"`
	Code     string                          `json:"code"`
	Name     string                          `json:"name"`
	Children map[string]CurrentCategoryChild `json:"children"`
}

// CurrentCategoryItem represents a single category item from the API
type CurrentCategoryItem struct {
	ID       int                             `json:"id"`
	Code     string                          `json:"code"`
	Name     string                          `json:"name"`
	Children map[string]CurrentCategoryChild `json:"children"`
}

// CurrentCategoryStructure represents the API response for get-categories
type CurrentCategoryStructure map[string]CurrentCategoryItem

var (
	catalogCategoryStructure map[string]*Category
	currentCategoryStructure CurrentCategoryStructure
	parentID                 int
	apiHost                  = "https://recom-power.cn/wp-json/recom-wordpress/v1/"
	
	// 统计变量
	addedCount   int // 新增类别计数
	updatedCount int // 更新类别计数
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
	
	// Initialize global variables.
	catalogCategoryStructure = make(map[string]*Category)
	currentCategoryStructure = make(CurrentCategoryStructure)
	parentID = 0

	fmt.Println("Starting category synchronization...")

	// Load the catalog into the memory.
	catalogData, err := ioutil.ReadFile("/usr/share/logstash/data/recom-categories.xml")
	if err != nil {
		fmt.Printf("Failed to read categories.xml: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Parse the catalog.
	var catalog XMLCatalog
	err = xml.Unmarshal(catalogData, &catalog)
	if err != nil {
		fmt.Printf("Failed to parse XML: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Build the category structure.
	for _, rootCategory := range catalog.RootCategory {
		addCategoriesToCatalog(rootCategory)
	}

	// Get and build current category structure.
	err = getCurrentCategories()
	if err != nil {
		fmt.Printf("Failed to get current categories: %v\n", err)
		return &Response{StatusCode: 500}, err
	}

	// Synchronize categories
	for code, category := range catalogCategoryStructure {
		var currentCategoryID = 0
		var parentCategoryID = 0

		if currentItem, exists := currentCategoryStructure[code]; exists {
			fmt.Printf("Category already exists: %s\n", category.Code)
			currentCategoryID = currentItem.ID
			parentCategoryID = currentCategoryID

			// Apply updates to category if necessary.
			if currentItem.Name != category.Name {
				fmt.Printf("Update category: %s\n", category.Code)
				err = updateCategory(currentCategoryID, category)
				if err != nil {
					return &Response{StatusCode: 500}, err
				}
				updatedCount++ // 增加更新计数
			}

			// Add child categories.
			for childCode, child := range category.Children {
				found := false
				var childID int
				
				// Check if child exists in current structure
				for _, currentChild := range currentItem.Children {
					if currentChild.Code == childCode {
						found = true
						childID = currentChild.ID
						
						// Apply updates to child if necessary.
						if currentChild.Name != child.Name {
							fmt.Printf("Update child: %s\n", child.Code)
							err = updateCategory(childID, child)
							if err != nil {
								return &Response{StatusCode: 500}, err
							}
							updatedCount++ // 增加更新计数
						}
						break
					}
				}

				if !found {
					// Add child.
					fmt.Printf("Add child: %s\n", child.Code)
					err = addCategory(child, parentCategoryID)
					if err != nil {
						return &Response{StatusCode: 500}, err
					}
					addedCount++ // 增加新增计数
				}
			}
		} else {
			// Add category.
			fmt.Printf("Add category: %s\n", category.Code)
			err = addCategory(category, parentCategoryID)
			if err != nil {
				return &Response{StatusCode: 500}, err
			}
			addedCount++ // 增加新增计数

			parentCategoryID = parentID

			// Add child categories.
			for _, child := range category.Children {
				fmt.Printf("Add child: %s\n", child.Code)
				err = addCategory(child, parentCategoryID)
				if err != nil {
					return &Response{StatusCode: 500}, err
				}
				addedCount++ // 增加新增计数
			}
		}
	}

	// Return success response.
	fmt.Println("Category synchronization completed successfully.")
	
	// 将统计结果写入日志文件
	logMessage := fmt.Sprintf("[%s] Category synchronization completed. Added: %d categories, Updated: %d categories\n", 
		time.Now().Format("2006-01-02 15:04:05"), addedCount, updatedCount)
	
	err = writeLog(logMessage)
	if err != nil {
		fmt.Printf("Failed to write log: %v\n", err)
	}
	
	return &Response{StatusCode: 200}, nil
}

func addCategoriesToCatalog(categories XMLCategory) {
	parentCategory := categories
	var parent string

	addCategoryToCatalog(parentCategory, parent)
	
	if len(parentCategory.SubCategory) > 0 {
		parent = parentCategory.Code

		for _, subCategory := range parentCategory.SubCategory {
			addCategoryToCatalog(subCategory, parent)
		}
	}
}

func addCategoryToCatalog(category XMLCategory, parent string) {
	// Set code.
	code := category.Code
	var name string

	// Set name.
	for _, categoryName := range category.Name {
		if categoryName.Language == "zh" {
			name = strings.TrimSpace(categoryName.Value) // 清理名称中的空格
			break
		}
	}

	if parent == "" {
		catalogCategoryStructure[code] = generateCategoryStructure(code, name)
	} else {
		if _, exists := catalogCategoryStructure[parent]; !exists {
			// Create parent if it doesn't exist
			catalogCategoryStructure[parent] = &Category{
				Code:     parent,
				Name:     "", // Will be set later when we process this category
				Children: make(map[string]*Category),
			}
		}
		catalogCategoryStructure[parent].Children[code] = generateCategoryStructure(code, name)
	}
}

func generateCategoryStructure(code, name string) *Category {
	return &Category{
		Code:     code,
		Name:     name,
		Children: make(map[string]*Category),
	}
}

func getCurrentCategories() error {
	fmt.Println("Fetching current categories from API...")
	resp, err := http.Get(apiHost + "get-categories")
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

	// Try to unmarshal directly first
	err = json.Unmarshal(body, &currentCategoryStructure)
	if err != nil {
		// If that fails, try to handle special cases
		currentCategoryStructure = make(CurrentCategoryStructure)
		tempMap := make(map[string]interface{})
		err = json.Unmarshal(body, &tempMap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal response: %v", err)
		}
		
		// Convert to our structure manually
		for key, value := range tempMap {
			valueBytes, _ := json.Marshal(value)
			var item CurrentCategoryItem
			json.Unmarshal(valueBytes, &item)
			currentCategoryStructure[key] = item
		}
	}

	fmt.Println("Current categories fetched successfully.")
	return nil
}

func addCategory(category *Category, parent int) error {
	fmt.Printf("Adding category: %s with parent ID: %d\n", category.Code, parent)

	url := apiHost + "add-category"
	payload := map[string]interface{}{
		"code":   category.Code,
		"name":   category.Name,
		"parent": parent,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
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

	// Parse the returned ID
	err = json.Unmarshal(body, &parentID)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %v", err)
	}

	fmt.Printf("Category %s added successfully with ID: %d\n", category.Code, parentID)
	return nil
}

func updateCategory(categoryID int, category *Category) error {
	fmt.Printf("Updating category ID: %d (%s)\n", categoryID, category.Code)

	url := apiHost + "update-category"
	payload := map[string]interface{}{
		"id":   categoryID,
		"name": category.Name,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create a request with POST method
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

	fmt.Printf("Category ID %d updated successfully.\n", categoryID)
	return nil
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