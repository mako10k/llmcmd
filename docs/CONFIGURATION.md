# Configuration Guide

This document explains how to configure llmcmd for optimal use.

## Configuration Sources

llmcmd supports multiple configuration sources with the following priority order:

1. **Command Line Options** (highest priority)
2. **Configuration File** (`~/.llmcmdrc`)
3. **Environment Variables** (lowest priority)

## Configuration File

### Location
- Default: `~/.llmcmdrc`
- Custom: Use `-c` or `--config` option

### Format
The configuration file uses simple key=value format:

```ini
# OpenAI API Configuration
openai_api_key=sk-your-api-key-here
openai_base_url=https://api.openai.com/v1
model=gpt-4o-mini
max_tokens=4096
temperature=0.1

# Rate Limiting & Security
max_api_calls=50
timeout_seconds=300
max_retries=3
retry_delay_ms=1000

# File Processing Limits
max_file_size=10485760    # 10MB in bytes
read_buffer_size=4096     # 4KB buffer

# Advanced Options
system_prompt=            # Custom system prompt (empty = default)
disable_tools=false       # Set to true to disable LLM tools
```

### Example Configuration

Copy the example configuration:

```bash
curl -sL https://raw.githubusercontent.com/mako10k/llmcmd/main/.llmcmdrc.example -o ~/.llmcmdrc
```

## Environment Variables

You can configure llmcmd using environment variables:

```bash
# Required
export OPENAI_API_KEY="sk-your-api-key-here"

# Optional
export LLMCMD_MODEL="gpt-4o-mini"
export LLMCMD_MAX_TOKENS="4096"
export LLMCMD_TEMPERATURE="0.1"
export LLMCMD_MAX_API_CALLS="50"
export LLMCMD_TIMEOUT="300"
export LLMCMD_MAX_FILE_SIZE="10485760"
export LLMCMD_READ_BUFFER_SIZE="4096"
export LLMCMD_MAX_RETRIES="3"
export LLMCMD_RETRY_DELAY="1000"
```

## Command Line Options

Override any configuration with command line options:

```bash
llmcmd --config ~/.llmcmdrc.production --verbose --stats "Process this data"
```

## Configuration Options Reference

### OpenAI API Settings

| Option | Default | Description |
|--------|---------|-------------|
| `openai_api_key` | - | Your OpenAI API key (required) |
| `openai_base_url` | `https://api.openai.com/v1` | OpenAI API base URL |
| `model` | `gpt-4o-mini` | OpenAI model to use |
| `max_tokens` | `4096` | Maximum tokens per response |
| `temperature` | `0.1` | Model temperature (0.0-2.0) |

### Security & Rate Limiting

| Option | Default | Description |
|--------|---------|-------------|
| `max_api_calls` | `50` | Maximum API calls per session |
| `timeout_seconds` | `300` | Request timeout (5 minutes) |
| `max_retries` | `3` | Retry attempts for failed requests |
| `retry_delay_ms` | `1000` | Delay between retries (ms) |

### File Processing

| Option | Default | Description |
|--------|---------|-------------|
| `max_file_size` | `10485760` | Maximum file size (10MB) |
| `read_buffer_size` | `4096` | Read buffer size (4KB) |

### Advanced Settings

| Option | Default | Description |
|--------|---------|-------------|
| `system_prompt` | _(empty)_ | Custom system prompt |
| `disable_tools` | `false` | Disable LLM tool usage |

## Troubleshooting

### Configuration Not Found

If `~/.llmcmdrc` doesn't exist, llmcmd will use default values and environment variables.

### Invalid Configuration

Use `--verbose` flag to see configuration loading details:

```bash
llmcmd --verbose "test"
```

### API Key Issues

1. Check if API key is set:
   ```bash
   echo $OPENAI_API_KEY
   ```

2. Verify in configuration file:
   ```bash
   cat ~/.llmcmdrc | grep openai_api_key
   ```

3. Test with explicit key:
   ```bash
   OPENAI_API_KEY="sk-..." llmcmd "test"
   ```

## Security Considerations

- **API Key Protection**: Never commit API keys to version control
- **File Permissions**: Ensure `~/.llmcmdrc` has restricted permissions:
  ```bash
  chmod 600 ~/.llmcmdrc
  ```
- **Rate Limiting**: Use `max_api_calls` to prevent excessive usage
- **File Size Limits**: Adjust `max_file_size` based on your needs

## Best Practices

1. **Environment-specific configs**: Use different config files for development/production
2. **Version control**: Add `.llmcmdrc` to `.gitignore`
3. **Monitoring**: Use `--stats` flag to monitor API usage
4. **Testing**: Use `--verbose` for debugging configuration issues
