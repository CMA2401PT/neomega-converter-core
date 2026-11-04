// Package items 为方块实体转换可完整表达的物品堆叠。
//
// ConvertItem 先用方块表解释块物品：带 Block compound 时用它，其次按名称和顶层
// Damage 查 legacy data value，最后只用名称，用转出的 Java 方块名作为物品 ID。
// 非方块物品通过 third_party 的物品表解析；旧版改名（scute、record_*）先归一。
// 调用方把返回 nil 的条目从输出列表中省略，避免留下缺少内容的物品。
//
// 药水种类和附魔名称使用内嵌顺序表，把 Bedrock 编号对应到 Java 名称。
// 药水的顶层 Damage 是种类编号，不是耐久；药水箭使用编号减一查表。
// 越界编号和 Java 无对应效果的 decay 会使整个物品被跳过。
//
// 组件按 Java 1.21.4 写出：附魔、耐久、自定义名称、lore、皮革颜色、无法破坏、
// 修复成本、容器内容和书本。附魔书使用 stored_enchantments；容器内容递归转换，
// 转不出的条目逐条跳过，容器本身保留。
//
// 成书保留标题、作者、代数和所有书页，可编辑书保留原文书页。
// 名称、lore 与成书共用 JSON 文本组件，§ 格式码解析成颜色与粗体等样式；
// 十六进制颜色码和字体切换尚未处理。
// 书页保留完整换行，告示牌的四行限制由告示牌规则处理。
//
// 物品表由 third_party 从 GeyserMC/mappings 生成并内嵌，来源、版本与复现方式
// 见 third_party/README.md。表里没有的名字仍然跳过，不会原样透传到 Java。
package items
