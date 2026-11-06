// Package converter 定义两种中间结果：JavaChunk 和 BedrockChunk。
//
// 每一种转换都是"读源 → 得到目标版本的中间结果 → 写目标"三段：
//
//	Bedrock 存档  --be2je-->  JavaChunk     --javaout-->     Java 存档
//	Java 存档     --je2be-->  BedrockChunk  --bedrockout-->  Bedrock 存档
//	Java 结构文件 --structure-->  JavaChunk（不跨版本）
//	mcstructure  --structure-->  BedrockChunk（不跨版本）
//
// be2je 和 je2be 只做跨版本的内容翻译，不碰文件；javaout 和 bedrockout 只管写成原生记录，
// 不关心数据从哪来。be2je/export 和 je2be/export 把读源、翻译和写出接成整图转换。
package converter
