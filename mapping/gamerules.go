package mapping

// GameRule 是一对两版同名的游戏规则。
type GameRule struct {
	Java    string
	Bedrock string
}

// GameRules 只列两版都存在的规则。
var GameRules = []GameRule{
	{"doDaylightCycle", "dodaylightcycle"},
	{"doWeatherCycle", "doweathercycle"},
	{"doFireTick", "dofiretick"},
	{"doInsomnia", "doinsomnia"},
	{"doMobSpawning", "domobspawning"},
	{"doMobLoot", "domobloot"},
	{"doTileDrops", "dotiledrops"},
	{"doEntityDrops", "doentitydrops"},
	{"doLimitedCrafting", "dolimitedcrafting"},
	{"doImmediateRespawn", "doimmediaterespawn"},
	{"keepInventory", "keepinventory"},
	{"mobGriefing", "mobgriefing"},
	{"naturalRegeneration", "naturalregeneration"},
	{"randomTickSpeed", "randomtickspeed"},
	{"playersSleepingPercentage", "playerssleepingpercentage"},
	{"projectilesCanBreakBlocks", "projectilescanbreakblocks"},
	{"pvp", "pvp"},
	{"spawnRadius", "spawnradius"},
	{"commandBlockOutput", "commandblockoutput"},
	{"sendCommandFeedback", "sendcommandfeedback"},
	{"showDeathMessages", "showdeathmessages"},
	{"maxCommandChainLength", "maxcommandchainlength"},
	{"fallDamage", "falldamage"},
	{"fireDamage", "firedamage"},
	{"drowningDamage", "drowningdamage"},
	{"freezeDamage", "freezedamage"},
	{"tntExplosionDropDecay", "tntexplosiondropdecay"},
}
