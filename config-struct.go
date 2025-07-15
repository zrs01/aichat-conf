package main

type ConfigStruct struct {
	// LLM
	Model       string  `yaml:"model,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
	// Behavior
	Stream      bool   `yaml:"stream,omitempty"`
	Save        bool   `yaml:"save,omitempty"`
	Keybindings string `yaml:"keybindings,omitempty"`
	Editor      string `yaml:"editor,omitempty"`
	Wrap        bool   `yaml:"wrap,omitempty"`
	WrapCode    bool   `yaml:"wrap_code,omitempty"`
	// Function calling
	FunctionCalling bool `yaml:"function_calling,omitempty"`
	UseTools        bool `yaml:"use_tools,omitempty"`
	// Prelude
	ReplPrelude  string `yaml:"repl_prelude,omitempty"`
	CmdPrelude   string `yaml:"cmd_prelude,omitempty"`
	AgentPrelude string `yaml:"agent_prelude,omitempty"`
	// Session
	SaveSession       bool   `yaml:"save_session,omitempty"`
	CompressThreshold int    `yaml:"compress_threshold,omitempty"`
	SummarizePrompt   string `yaml:"summarize_prompt,omitempty"`
	SummaryPrompt     string `yaml:"summary_prompt,omitempty"`
	// RAG
	RagEmbeddingModel string `yaml:"rag_embedding_model,omitempty"`
	RegRerankerModel  string `yaml:"reg_reranker_model,omitempty"`
	RagTopK           int    `yaml:"rag_top_k,omitempty"`
	RagChunkSize      int    `yaml:"rag_chunk_size,omitempty"`
	RagChunkOverlap   int    `yaml:"rag_chunk_overlap,omitempty"`
	RagTemplate       string `yaml:"rag_template,omitempty"`
	// Appearance
	Highlight   bool   `yaml:"highlight,omitempty"`
	Theme       string `yaml:"theme,omitempty"`
	LeftPrompt  string `yaml:"left_prompt,omitempty"`
	RightPrompt string `yaml:"right_prompt,omitempty"`
	// Misc
	ServeAddr        string `yaml:"serve_addr,omitempty"`
	UserAgent        string `yaml:"user_agent,omitempty"`
	SaveShellHistory bool   `yaml:"save_shell_history,omitempty"`
	SyncModelsUrl    string `yaml:"sync_models_url,omitempty"`
	// Client
	Clients      []Client `yaml:"clients,omitempty"`
	MappingTools struct {
		Fs string `yaml:"fs,omitempty"`
	} `yaml:"mapping_tools,omitempty"`
	DocumentLoaders struct {
		Pdf  string `yaml:"pdf,omitempty"`
		Docx string `yaml:"docx,omitempty"`
	} `yaml:"document_loaders,omitempty"`
}

type Client struct {
	Type    string        `yaml:"type,omitempty"`
	Name    string        `yaml:"name,omitempty"`
	ApiBase string        `yaml:"api_base,omitempty"`
	ApiKey  string        `yaml:"api_key,omitempty"`
	Models  []ClientModel `yaml:"models,omitempty"`
}

type ClientModel struct {
	Name                    string  `yaml:"name,omitempty"`
	Temperature             float64 `yaml:"temperature,omitempty"`
	TopP                    float64 `yaml:"top_p,omitempty"`
	MaxInputTokens          int     `yaml:"max_input_tokens,omitempty"`
	SupportsVision          bool    `yaml:"supports_vision,omitempty"`
	SupportsFunctionCalling bool    `yaml:"supports_function_calling,omitempty"`
	SupportsReasoning       bool    `yaml:"supports_reasoning,omitempty"`
	NoStream                bool    `yaml:"no_stream,omitempty"`
	NoSystemMessage         bool    `yaml:"no_system_message,omitempty"`
	SystemPromptPrefix      string  `yaml:"system_prompt_prefix,omitempty"`
	// Embedding
	DefaultChunkSize  int `yaml:"default_chunk_size,omitempty"`
	MaxBatchSize      int `yaml:"max_batch_size,omitempty"`
	MaxTokensPerChunk int `yaml:"max_tokens_per_chunk,omitempty"`
	// Extra
	Extra struct {
		Proxy string `yaml:"proxy,omitempty"`
	} `yaml:"extra,omitempty"`
	// Patch
	Patch any `yaml:"patch,omitempty"`
}
