const MODEL_FAMILY_RULES = [
  ['OpenAI', /^(gpt|chatgpt|o[134](?:-|$)|dall-e|sora|codex)/i],
  ['Claude', /^claude/i],
  ['Gemini', /^(gemini|gemma|imagen|veo|nano[ -]?banana)/i],
  ['Grok', /^grok/i],
  ['DeepSeek', /^deepseek/i],
  ['Qwen', /^(qwen|qwq)/i],
  ['ByteDance', /^(doubao|seedream|seedance)/i],
  ['Zhipu', /^(glm|cogview|cogvideo)/i],
  ['Kimi', /^(kimi|moonshot)/i],
  ['MiniMax', /^minimax/i],
  ['Mistral', /^(mistral|codestral|ministral|pixtral)/i],
  ['Meta', /^(llama|meta-llama)/i],
  ['Cohere', /^(command|c4ai)/i],
  ['Baidu', /^(ernie|wenxin)/i],
  ['Hunyuan', /^hunyuan/i],
];

export const getModelFamily = (name = '') =>
  MODEL_FAMILY_RULES.find(([, pattern]) => pattern.test(name))?.[0] || 'Other';
