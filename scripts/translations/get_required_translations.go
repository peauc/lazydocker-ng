package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/peauc/lazydocker-ng/pkg/i18n"
)

func main() {
	fmt.Println(getOutstandingTranslations())
}

// adapted from https://github.com/a8m/reflect-examples#read-struct-tags
func getOutstandingTranslations() string {
	var output strings.Builder
	for languageCode, translationSet := range i18n.GetTranslationSets() {
		output.WriteString(languageCode + ":\n")
		v := reflect.ValueOf(translationSet)

		for i := 0; i < v.NumField(); i++ {
			value := v.Field(i).String()
			if value == "" {
				output.WriteString(v.Type().Field(i).Name + "\n")
			}
		}
		output.WriteString("\n")
	}
	return output.String()
}
