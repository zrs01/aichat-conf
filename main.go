// Package main is the entry point of the application.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/ollama/ollama/api"
	olmapi "github.com/ollama/ollama/api"
	olmmodel "github.com/ollama/ollama/types/model"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"github.com/ztrue/tracerr"
	"gopkg.in/yaml.v3"
)

var (
	version       string
	optDebug      bool
	optQuiet      bool
	optCfgFile    string
	optClientName string
	optOutFile    string
	optExclude    string // models exclude
	optDefModel   string // default model
	ollamaClient  *olmapi.Client
)

func main() {
	initLogrus()

	cmd := &cli.Command{
		Name:    "aichatconf",
		Usage:   "A simple configuration tool for github.com/sigoden/aichat",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Required:    true,
				Usage:       "config file of aichat",
				Destination: &optCfgFile,
			},
			&cli.StringFlag{
				Name:        "client",
				Aliases:     []string{"n"},
				Usage:       "client name",
				Destination: &optClientName,
			},
			&cli.StringFlag{
				Name:        "model",
				Aliases:     []string{"m"},
				Usage:       "default model",
				Destination: &optDefModel,
			},
			&cli.StringFlag{
				Name:        "exclude",
				Aliases:     []string{"e"},
				Usage:       "models exclude, split by comma",
				Destination: &optExclude,
			},
			&cli.StringFlag{
				Name:        "output",
				Aliases:     []string{"o"},
				Usage:       "output file, default is stdout",
				Destination: &optOutFile,
			},
			&cli.BoolFlag{
				Name:        "quiet",
				Aliases:     []string{"q"},
				Value:       false,
				Usage:       "suppress all information output",
				Destination: &optQuiet,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Aliases:     []string{"d"},
				Required:    false,
				Usage:       "enable debug mode",
				Destination: &optDebug,
			},
		},
		Action: func(context.Context, *cli.Command) error {
			if optDebug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			return process()
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		if optDebug {
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
	verboseInfo("aichat configuration read: %s", optCfgFile)
	cfgBody, err := os.ReadFile(optCfgFile)
	if err != nil {
		return tracerr.Wrap(err)
	}
	// prepend "---" to the file if missing to preserve first line comments in YAML after unmarshal
	if len(cfgBody) >= 3 && string(cfgBody[:3]) != "---" {
		cfgBody = []byte("---\n" + string(cfgBody))
	}

	// use yaml.Node type to unmarshal in order to keep the comment
	var cfgDocNode yaml.Node
	if err := yaml.Unmarshal(cfgBody, &cfgDocNode); err != nil {
		return tracerr.Wrap(err)
	}
	if len(cfgDocNode.Content) == 0 {
		return tracerr.New("empty config file")
	}

	// find the default client and model
	var cfgDefModelClient, cfgDefModelName string
	var cfgDefModelNode *yaml.Node
	{
		node, ok := getNodeValue(cfgDocNode.Content[0], "model", yaml.ScalarNode)
		if ok {
			re := regexp.MustCompile(`^([^:]+):(.+)$`)
			match := re.FindStringSubmatch(node.Value)
			if len(match) > 2 {
				cfgDefModelNode = node
				cfgDefModelClient = strings.TrimSpace(match[1])
				cfgDefModelName = strings.TrimSpace(match[2])
			}
		}
	}

	verboseInfo("default model found: %s:%s", cfgDefModelClient, cfgDefModelName)
	// find the clients
	cfgClients, _ := getNodeValue(cfgDocNode.Content[0], "clients", yaml.SequenceNode)
	var cfgOllamaClient *yaml.Node = nil
	verboseInfo("clients found: %d", len(cfgClients.Content))

	// find the ollama client and its models
	if optClientName == "" {
		// use client in the model as default if user does not provided
		optClientName = cfgDefModelClient
	}
	cfgOllamaModels := &yaml.Node{}
	for _, cn := range cfgClients.Content {
		for j, node := range cn.Content {
			if node.Kind == yaml.ScalarNode && node.Value == "name" {
				if cn.Content[j+1].Kind == yaml.ScalarNode && cn.Content[j+1].Value == optClientName {
					cfgOllamaClient = cn
					cfgOllamaModels, _ = getNodeValue(cn, "models", yaml.SequenceNode)
					verboseInfo("models found: %d", len(cfgOllamaModels.Content))
				}
			}
		}
	}
	if cfgOllamaClient == nil {
		return tracerr.Errorf("ollama client name (%s) not found", optClientName)
	}

	// create ollama client
	{
		cfgOllamaAPIKey := ""
		if apiKeyNode, ok := getNodeValue(cfgOllamaClient, "api_key", yaml.ScalarNode); ok {
			cfgOllamaAPIKey = apiKeyNode.Value
			verboseInfo("api_key found")
		}

		cfgOllamaAPIBase := ""
		if apiBaseNode, ok := getNodeValue(cfgOllamaClient, "api_base", yaml.ScalarNode); ok {
			cfgOllamaAPIBase = apiBaseNode.Value
			verboseInfo("api_base found: %s", cfgOllamaAPIBase)
		} else {
			verboseInfo("api_base not found, use default")
		}
		c, err := createOllamaClient(cfgOllamaAPIBase, cfgOllamaAPIKey)
		if err != nil {
			return tracerr.Wrap(err)
		}
		ollamaClient = c
	}

	/* -------------------------------------------------------------------------- */
	/*                                OLLAMA MODELS                               */
	/* -------------------------------------------------------------------------- */
	ollamaModels, err := getOllamaModels()
	if err != nil {
		return tracerr.Wrap(err)
	}
	verboseInfo("ollama models found: %d", len(ollamaModels))
	// exclude models
	if optExclude != "" {
		excludeModels := strings.Split(optExclude, ",")
		lo.ForEach(excludeModels, func(model string, _ int) {
			model = strings.TrimSpace(model)
		})
		ollamaModels = lo.Filter(ollamaModels, func(model string, _ int) bool {
			for _, excludeModel := range excludeModels {
				if strings.Contains(model, excludeModel) {
					verboseInfo("exclude model: %s", model)
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
				} else {
					verboseInfo("remove model: %s", cfgModelName.Value)
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
				maxCtxLen, temperature, topP, capabilities, err := getModelParameters(model)
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
				if lo.Contains(capabilities, olmmodel.CapabilityVision) {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "supports_vision"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "true"})
				}
				if lo.Contains(capabilities, olmmodel.CapabilityTools) {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "supports_function_calling"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "true"})
				}
				if lo.Contains(capabilities, olmmodel.CapabilityThinking) {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "supports_reasoning"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "true"})
				}
				if lo.Contains(capabilities, olmmodel.CapabilityEmbedding) {
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "type"})
					newNode.Content = append(newNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "embedding"})
				}
				cfgOllamaModels.Content = append(cfgOllamaModels.Content, newNode)
				verboseInfo("add model: %s", model)
			}
		}
	}
	// sort the models by name
	sort.Slice(cfgOllamaModels.Content, func(a, b int) bool {
		aName, _ := getNodeValue(cfgOllamaModels.Content[a], "name", yaml.ScalarNode)
		bName, _ := getNodeValue(cfgOllamaModels.Content[b], "name", yaml.ScalarNode)
		return aName.Value < bName.Value
	})
	if optDefModel != "" {
		var desiredModel string
		for _, cfgModel := range cfgOllamaModels.Content {
			cfgModelName, ok := getNodeValue(cfgModel, "name", yaml.ScalarNode)
			if ok {
				if strings.Contains(cfgModelName.Value, optDefModel) {
					desiredModel = cfgModelName.Value
					break
				}
			}
		}
		if desiredModel != "" {
			cfgDefModelName = fmt.Sprintf("%s:%s", optClientName, desiredModel)
			cfgDefModelNode.Value = fmt.Sprintf("%s:%s", optClientName, desiredModel)
			verboseInfo("set default model: %s", cfgDefModelName)
		} else {
			verboseInfo("default model setting skip, model not found: %s", optDefModel)
		}
	}

	/* -------------------------------------------------------------------------- */
	/*                                   OUTPUT                                   */
	/* -------------------------------------------------------------------------- */
	outbytes, err := yaml.Marshal(cfgDocNode.Content[0])
	if err != nil {
		return tracerr.Wrap(err)
	}
	outstr := strings.TrimSpace(string(outbytes))
	if optOutFile != "" {
		verboseInfo("write to: %s", optOutFile)
		return os.WriteFile(optOutFile, []byte(outstr), 0644)
	} else {
		verboseInfo("write to: stdout")
		fmt.Printf("%s\n", string(outstr))
	}

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

func initLogrus() {
	logrus.SetFormatter(&nested.Formatter{
		HideKeys:        true,
		TimestampFormat: time.RFC3339,
	})
}

func verboseInfo(format string, args ...any) {
	if !optQuiet {
		logrus.Infof(format, args...)
	}
}

func getOllamaModels() ([]string, error) {
	resp, err := ollamaClient.List(context.Background())
	if err != nil {
		return []string{}, tracerr.Wrap(err)
	}
	models := lo.Map(resp.Models, func(model olmapi.ListModelResponse, _ int) string {
		return model.Name
	})
	return models, nil
}

func getModelParameters(model string) (int, float64, float64, []olmmodel.Capability, error) {
	maxContextLength := -1
	temperature := -1.0
	topP := -1.0

	info, err := getModelInfo(model)
	if err != nil {
		return maxContextLength, temperature, topP, nil, tracerr.Wrap(err)
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
	return maxContextLength, temperature, topP, info.Capabilities, nil
}

func getModelInfo(model string) (*olmapi.ShowResponse, error) {
	resp, err := ollamaClient.Show(context.Background(), &olmapi.ShowRequest{Model: model})
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return resp, nil
}

/* -------------------------------------------------------------------------- */
/*                     OLLAMA CLIENT WITH API KEY SUPPORT                     */
/* -------------------------------------------------------------------------- */

// apiKeyTransport adds the API_KEY header to every request.
type apiKeyTransport struct {
	rt     http.RoundTripper // the underlying transport
	apiKey string            // the value you want to send
}

// RoundTrip implements http.RoundTripper.
func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request so we don't mutate the caller's request
	// (recommended by the net/http docs for RoundTripper wrappers).
	req2 := req.Clone(req.Context())

	// Add the header – you can use Add, Set or Direct assignment.
	req2.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

	// Pass the request on to the wrapped RoundTripper.
	return t.rt.RoundTrip(req2)
}

func createOllamaClient(apiBase, apiKey string) (*api.Client, error) {
	// Use http.DefaultTransport if you don't need custom TLS settings.
	// If you do need TLS or proxy config, create your own *http.Transport.
	base := http.DefaultTransport

	// Wrap it
	wrapped := &apiKeyTransport{
		rt:     base,
		apiKey: apiKey,
	}

	httpClient := &http.Client{
		Transport: wrapped,
	}

	var client *api.Client
	if apiBase != "" {
		// remove the path
		u, err := url.Parse(apiBase)
		if err != nil {
			return nil, tracerr.Wrap(err)
		}
		u.Path = ""
		client = olmapi.NewClient(u, httpClient)
	} else {
		c, err := olmapi.ClientFromEnvironment()
		if err != nil {
			return nil, tracerr.Wrap(err)
		}
		client = c
	}
	return client, nil
}
