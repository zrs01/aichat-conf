// Package main is the entry point of the application.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/ollama/ollama/api"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"github.com/ztrue/tracerr"
	"gopkg.in/yaml.v3"
)

var (
	version string
	debug   bool
	cfgFile string
	exclude string // models exclude
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
				Destination: &cfgFile,
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
	/* -------------------------------------------------------------------------- */
	/*                          READ AICHAT CONFIGURATION                         */
	/* -------------------------------------------------------------------------- */
	cfgBody, err := os.ReadFile(cfgFile)
	if err != nil {
		return tracerr.Wrap(err)
	}
	// use yaml.Node type to unmarshal in order to keep the comment
	var cfgDocNode yaml.Node
	if err := yaml.Unmarshal(cfgBody, &cfgDocNode); err != nil {
		return tracerr.Wrap(err)
	}
	if len(cfgDocNode.Content) == 0 {
		return tracerr.New("empty config file")
	}
	// find the clients
	cfgClientNode, _ := getNodeValue(cfgDocNode.Content[0], "clients", yaml.SequenceNode)

	cfgOllamaModels := &yaml.Node{}
	for _, cn := range cfgClientNode.Content {
		for j, node := range cn.Content {
			if node.Kind == yaml.ScalarNode && node.Value == "name" {
				if cn.Content[j+1].Kind == yaml.ScalarNode && cn.Content[j+1].Value == "ollama" {
					cfgOllamaModels, _ = getNodeValue(cn, "models", yaml.SequenceNode)
				}
			}
		}
	}

	/* -------------------------------------------------------------------------- */
	/*                                OLLAMA MODELS                               */
	/* -------------------------------------------------------------------------- */
	ollamaModels, err := getOllamaModels()
	if err != nil {
		return tracerr.Wrap(err)
	}
	// exclude models
	if exclude != "" {
		excludeModels := strings.Split(exclude, ",")
		lo.ForEach(excludeModels, func(model string, _ int) {
			model = strings.TrimSpace(model)
		})
		ollamaModels = lo.Filter(ollamaModels, func(model string, _ int) bool {
			for _, excludeModel := range excludeModels {
				if strings.Contains(model, excludeModel) {
					return false
				}
			}
			return true
		})
	}

	// remove obsolete models
	{
		newModels := []*yaml.Node{}
		for _, cfgModel := range cfgOllamaModels.Content {
			cfgModelName, ok := getNodeValue(cfgModel, "name", yaml.ScalarNode)
			if ok {
				if lo.Contains(ollamaModels, cfgModelName.Value) {
					newModels = append(newModels, cfgModel)
				}
			}
		}
		cfgOllamaModels.Content = newModels
	}
	// add new models
	{
		for _, model := range ollamaModels {
			found := false
			for _, cfgModel := range cfgOllamaModels.Content {
				cfgModelName, ok := getNodeValue(cfgModel, "name", yaml.ScalarNode)
				if ok && cfgModelName.Value == model {
					found = true
					break
				}
			}
			if !found {
				maxCtxLen, temperature, topP, err := getModelParameters(model)
				if err != nil {
					tracerr.Wrap(err)
				}
				newNode := &yaml.Node{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "name"},
						{Kind: yaml.ScalarNode, Value: model},
					},
				}
				if maxCtxLen > 0 {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "max_input_tokens"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(maxCtxLen)})
				}
				if temperature > 0 {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "temperature"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatFloat(temperature, 'f', 1, 64)})
				}
				if topP > 0 {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "top_p"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatFloat(topP, 'f', 1, 64)})
				}
				cfgOllamaModels.Content = append(cfgOllamaModels.Content, newNode)
			}
		}
	}

	/* -------------------------------------------------------------------------- */
	/*                         UPDATE AICHAT CONFIGURATION                        */
	/* -------------------------------------------------------------------------- */
	for i, cn := range cfgClientNode.Content {
		for j, node := range cn.Content {
			if node.Kind == yaml.ScalarNode && node.Value == "name" {
				if cn.Content[j+1].Kind == yaml.ScalarNode && cn.Content[j+1].Value == "ollama" {
					setNodeValue(cfgClientNode.Content[i].Content[j+2], "models", cfgOllamaModels)
				}
			}
		}
	}

	/* -------------------------------------------------------------------------- */
	/*                                   OUTPUT                                   */
	/* -------------------------------------------------------------------------- */
	out, err := yaml.Marshal(cfgDocNode.Content[0])
	if err != nil {
		return tracerr.Wrap(err)
	}
	fmt.Printf("\n%s", string(out))

	return nil
}

func getNodeValue(node *yaml.Node, key string, valueKind yaml.Kind) (*yaml.Node, bool) {
	for i, childNode := range node.Content {
		if childNode.Kind == yaml.ScalarNode && childNode.Value == key {
			if i+1 < len(node.Content) {
				if node.Content[i+1].Kind == valueKind {
					return node.Content[i+1], true
				}
			}
		}
	}
	return &yaml.Node{Kind: valueKind}, false
}

func setNodeValue(node *yaml.Node, key string, value *yaml.Node) {
	for i, childNode := range node.Content {
		if childNode.Kind == yaml.ScalarNode && childNode.Value == key {
			if i+1 < len(node.Content) {
				node.Content[i+1] = value
				return
			}
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key})
	node.Content = append(node.Content, value)
}

func initLogrus() {
	logrus.SetFormatter(&nested.Formatter{
		HideKeys:        true,
		TimestampFormat: time.RFC3339,
	})
}

func getOllamaModels() ([]string, error) {
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

func getModelParameters(model string) (int, float64, float64, error) {
	maxContextLength := -1
	temperature := -1.0
	topP := -1.0

	info, err := getModelInfo(model)
	if err != nil {
		return maxContextLength, temperature, topP, tracerr.Wrap(err)
	}
	// find the max context length
	for key, value := range info.ModelInfo {
		if strings.Contains(key, ".context_length") {
			maxContextLength = int(value.(float64))
			break
		}
	}
	// find temperature and top_p
	parameters := strings.SplitSeq(info.Parameters, "\n")
	for parameter := range parameters {
		paramKV := strings.Fields(parameter)
		if len(paramKV) > 1 {
			paramValue := strings.TrimSpace(paramKV[1])
			if strings.Contains(paramKV[0], "temperature") {
				f, err := strconv.ParseFloat(paramValue, 64)
				if err == nil {
					temperature = f
				}
			}
			if strings.Contains(paramKV[0], "top_p") {
				f, err := strconv.ParseFloat(paramValue, 64)
				if err == nil {
					topP = f
				}
			}
		}
	}
	return maxContextLength, temperature, topP, nil
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
