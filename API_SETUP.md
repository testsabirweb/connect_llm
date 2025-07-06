# API Key Setup Guide for ConnectLLM

This guide explains how to configure API keys for both Taskmaster and the ConnectLLM application.

## ✅ Current Configuration Status

**OpenRouter Configuration Required:**

- API Key: Need to configure in `.cursor/mcp.json`
- Provider: OpenRouter
- Models: All set to `deepseek/deepseek-chat-v3-0324:free` (FREE tier!)
- Status: Requires new API key after security incident

⚠️ **Important**: After updating API keys in `.cursor/mcp.json`, you must restart Cursor for the changes to take effect. The MCP server needs to reload with the new environment variables.

## Taskmaster AI Configuration

Taskmaster can use various AI providers. Currently configured to use DeepSeek through OpenRouter.

### Option 1: Using OpenRouter (Recommended)

OpenRouter provides access to multiple AI models including DeepSeek. To use it:

1. **Get an OpenRouter API Key**:
   - Visit [OpenRouter](https://openrouter.ai)
   - Sign up for an account
   - Generate an API key from your dashboard
   - OpenRouter keys typically start with `sk-or-`

2. **Configure for MCP (Cursor)**:
   - Update the key in `.cursor/mcp.json`:

     ```json
     {
       "mcpServers": {
         "taskmaster-ai": {
           "env": {
             "OPENROUTER_API_KEY": "sk-or-your-key-here"
           }
         }
       }
     }
     ```

3. **Configure for CLI**:
   - Create a `.env` file in the project root:

     ```bash
     OPENROUTER_API_KEY=sk-or-your-key-here
     ```

### Option 2: Direct DeepSeek API

If you have a direct DeepSeek API key:

**Note**: Taskmaster doesn't currently support direct DeepSeek API keys. You'll need to:

- Use OpenRouter as a proxy (recommended)
- Or configure a custom model through Ollama

### Verifying Configuration

```bash
# Check current model configuration
tm models

# The output should show:
# - main: deepseek/deepseek-chat-v3-0324 (openrouter)
# - keyStatus.mcp: true (if properly configured)
```

## API Key Security Best Practices

1. **Never commit API keys** to version control
2. **Use environment variables** or secure configuration files
3. **Rotate keys regularly** - See [API key rotation best practices](https://www.traceable.ai/blog-post/dizzy-keys-why-api-key-rotation-matters)
4. **Limit key permissions** to only what's needed
5. **Monitor key usage** for unusual activity

## Troubleshooting

### "AI service call failed for all configured roles"

This error usually means:

1. The API key is invalid or expired
2. The API key is for the wrong service (e.g., DeepSeek key used with OpenRouter)
3. The API service is temporarily unavailable

**Solution**:

- Verify you're using an OpenRouter API key (not a direct DeepSeek key)
- Check the key is properly formatted in `.cursor/mcp.json`
- Test the key works by visiting OpenRouter's dashboard

### For ConnectLLM Application

The main application will use different APIs:

1. **Ollama** - Already configured in Docker, no API key needed
2. **Weaviate** - No API key needed for local deployment
3. **Future Connectors**:
   - Slack: Will need OAuth app credentials
   - GitHub: Will need personal access token or OAuth app
   - Jira: Will need API token
   - Google Workspace: Will need service account credentials

These will be configured in the application's own `.env` file when implemented.
