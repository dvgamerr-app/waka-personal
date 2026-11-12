# ราคาโมเดล AI

อัปเดต 2026-11-12 ผ่าน `20261112120000_ai_model_pricing_all_providers.sql`
ราคาเป็น USD ต่อ 1 ล้าน tokens แยก input/output โดยเก็บโมเดลเดิมไว้สำหรับประวัติ

| ช่องทาง    |          รายการ | การจับคู่                                                          |
| ---------- | --------------: | ------------------------------------------------------------------ |
| API ตรง    |              88 | model ID เช่น `glm-5.3`, `gpt-6.1-terra`                           |
| OpenRouter | 460 จาก 68 ค่าย | ใช้ namespace เต็ม เช่น `moonshotai/kimi-k3.1-pro`, `z-ai/glm-5.3` |

API ตรงครอบคลุม OpenAI, Anthropic, Google, Z.ai/GLM, DeepSeek, Alibaba/Qwen,
xAI, MiniMax และ Mistral ส่วน OpenRouter รวม Kimi, Cohere, Amazon, Meta,
NVIDIA, Xiaomi, ByteDance และค่ายอื่นที่มีโมเดลคืนค่า text และมีราคา token ที่ไม่ติดลบ
ราคา OpenRouter เป็นราคาของ gateway อาจต่างจากราคา API ตรงและเปลี่ยนตามผู้ให้บริการที่เลือก
โมเดลที่คิดราคาต่อภาพ/วินาที/คำขอ ไม่รวมค่าใช้จ่ายส่วนนั้นในตารางนี้

| ราคา input/output ที่เปลี่ยน | เดิม         | ใหม่                                      |
| ---------------------------- | ------------ | ----------------------------------------- |
| GPT-5.6 Sol                  | 5 / 30       | 4 / 20 (โปรโมชันถึง 2026-11-30)           |
| GPT-5.6 Terra                | 2.5 / 15     | 2 / 12                                    |
| GPT-5.6 Luna                 | 1 / 6        | 0.2 / 1.2                                 |
| o3                           | 10 / 40      | 2 / 8                                     |
| Claude (ไม่ระบุโมเดล)        | 3 / 15       | 2 / 10 (ประมาณด้วย Sonnet 5.5)            |
| Codex (ไม่ระบุโมเดล)         | 1.75 / 14    | 2 / 10 (ประมาณด้วย GPT-6.1 Sol)           |
| Gemini (ไม่ระบุโมเดล)        | 0.075 / 0.30 | 0.75 / 3.75 (ประมาณด้วย Gemini 3.8 Flash) |

Dashboard อ่านราคาใน DB ทั้ง API ingest และ importer ใช้เส้นทางคำนวณเดียวกัน
plugin รองรับ model ID โดยตรง เช่น `opencode/1 glm-5.3`,
`cline/1 model=moonshotai/kimi-k3.1-pro` และแบบ `glm/5.3`
รูปแบบเดิม `sonnet/5.5` และ `gpt/6.1-sol-high` ยังใช้ได้
หากไม่มี model ID ใช้ชื่อ agent หรืออัตรา default เดิม

ตัวเลขเป็นประมาณการด้วยอัตราปัจจุบันทั้งประวัติ ไม่ใช่ใบแจ้งหนี้:
ยังไม่แยก cache, batch, priority, region, tools, subscription หรือ context tier
DeepSeek ใช้ราคา peak; Qwen ใช้ Singapore International; เงื่อนไขแต่ละรายการอยู่ใน `notes`
Gemini 4 Carbon ใช้โปรโมชัน 2/10 ถึง 2026-11-30 และจะเป็น 4/20 ตั้งแต่ 2026-12-01
Gemini 3.8 Flash ใช้ 0.75/3.75 ถึง 2026-12-31 และจะเป็น 1.50/7.50 ตั้งแต่ 2027-01-01
ต้องอัปเดตราคาอีกครั้งเมื่อโปรโมชันสิ้นสุด ตารางนี้ไม่เปลี่ยนราคาอัตโนมัติ

นำไปใช้ด้วย `go run ./cmd/migrate up` หรือ startup เมื่อ `AUTO_MIGRATE=true`
rollback ด้วย goose Down จะคืนค่า seed เดิมและลบ model keys ที่เพิ่มรอบนี้
หากมีการแก้ราคาเองใน DB ต้องสำรองค่าก่อน rollback เพราะ Down คืนค่า seed ตาม repository

แหล่งข้อมูลที่ตรวจ:

- [OpenAI Docs](https://developers.openai.com/api/docs/pricing.md)
- [Anthropic](https://platform.claude.com/docs/en/about-claude/pricing)
- [Google](https://ai.google.dev/gemini-api/docs/pricing)
- [Z.ai / GLM](https://docs.z.ai/guides/overview/pricing)
- [DeepSeek](https://api-docs.deepseek.com/quick_start/pricing/)
- [Alibaba / Qwen](https://www.alibabacloud.com/help/en/model-studio/model-pricing)
- [xAI](https://docs.x.ai/developers/models)
- [MiniMax](https://platform.minimax.io/docs/pricing/overview)
- [Mistral](https://docs.mistral.ai/inference/pricing)
- [OpenRouter model catalog](https://openrouter.ai/api/v1/models)
