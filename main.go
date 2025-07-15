// Package main is the entry point of the application.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/ollama/ollama/api"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"github.com/ztrue/tracerr"
	"gopkg.in/yaml.v2"
)

var (
	version    string
	debug      bool
	configFile string
	exclude    string // models exclude
)

func main() {
	initLogrus()

	cmd := &cli.Command{
		Name:    "aichatconf",
		Usage:   "A simple configuration tool for github.com/sigoden/aichat",
		Version: version,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "debug",
				Aliases:     []string{"d"},
				Value:       false,
				Usage:       "toggle debug message",
				Destination: &debug,
			},
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Required:    true,
				Usage:       "config file of aichat",
				Destination: &configFile,
			},
			&cli.StringFlag{
				Name:        "exclude",
				Aliases:     []string{"e"},
				Usage:       "models exclude, split by comma",
				Destination: &exclude,
			},
		},
		Action: func(context.Context, *cli.Command) error {
			return process()
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		if debug {
			logrus.Error(tracerr.SprintSourceColor(err, 0))
		} else {
			logrus.Error(err)
		}
		os.Exit(1)
	}
}

func process() error {
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return tracerr.Wrap(err)
	}
	var aichatConf ConfigStruct
	if err := yaml.Unmarshal(yamlFile, &aichatConf); err != nil {
		return tracerr.Wrap(err)
	}
	// find ollama provider
	client, index, found := lo.FindIndexOf(aichatConf.Clients, func(c Client) bool {
		return c.Name == "ollama"
	})
	if !found {
		logrus.Info("ollama provider not found")
		return nil
	}
	modelList, err := getModelList()
	if err != nil {
		return tracerr.Wrap(err)
	}

	// exclude models
	if exclude != "" {
		excludeModels := strings.Split(exclude, ",")
		lo.ForEach(excludeModels, func(model string, _ int) {
			model = strings.TrimSpace(model)
		})
		modelList = lo.Filter(modelList, func(model string, _ int) bool {
			for _, excludeModel := range excludeModels {
				if strings.Contains(model, excludeModel) {
					return false
				}
			}
			return true
		})
	}

	// iterated the model list, add the missing model to aichat configuration
	for _, model := range modelList {
		_, found := lo.Find(client.Models, func(m ClientModel) bool {
			return m.Name == model
		})
		if !found {
			info, err := getModelInfo(model)
			if err != nil {
				return tracerr.Wrap(err)
			}
			// o, _ := yaml.Marshal(info)
			// fmt.Println(string(o))

			newModel := ClientModel{Name: model}

			// find the max context length
			for key, value := range info.ModelInfo {
				if strings.Contains(key, ".context_length") {
					maxContextLength := int(value.(float64))
					if maxContextLength > 0 {
						newModel.MaxInputTokens = maxContextLength
					}
					break
				}
			}
			// find parameters
			parameters := strings.SplitSeq(info.Parameters, "\n")
			for parameter := range parameters {
				paramKV := strings.Fields(parameter)
				if len(paramKV) > 1 {
					paramValue := strings.TrimSpace(paramKV[1])
					if strings.Contains(paramKV[0], "temperature") {
						f, err := strconv.ParseFloat(paramValue, 64)
						if err == nil {
							newModel.Temperature = f
						}
					}
					if strings.Contains(paramKV[0], "top_p") {
						f, err := strconv.ParseFloat(paramValue, 64)
						if err == nil {
							newModel.TopP = f
						}
					}
				}
			}
			client.Models = append(client.Models, newModel)
		}
	}
	sort.Slice(client.Models, func(i, j int) bool {
		return client.Models[i].Name < client.Models[j].Name
	})
	aichatConf.Clients[index] = client

	out, err := yaml.Marshal(aichatConf)
	if err != nil {
		return tracerr.Wrap(err)
	}
	fmt.Printf("\n%s", string(out))
	return nil
}

func initLogrus() {
	logrus.SetFormatter(&nested.Formatter{
		HideKeys:        true,
		TimestampFormat: time.RFC3339,
	})
}

func getModelList() ([]string, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return []string{}, tracerr.Wrap(err)
	}
	resp, err := client.List(context.Background())
	if err != nil {
		return []string{}, tracerr.Wrap(err)
	}
	models := lo.Map(resp.Models, func(model api.ListModelResponse, _ int) string {
		return model.Name
	})
	return models, nil
}

func getModelInfo(model string) (*api.ShowResponse, error) {
	client, err := api.ClientFromEnvironment()
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	resp, err := client.Show(context.Background(), &api.ShowRequest{Model: model})
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return resp, nil
}
