package mining

// Richness represents the resource abundance of a mining node.
type Richness string

const (
	RichnessPoor    Richness = "poor"
	RichnessNormal  Richness = "normal"
	RichnessRich    Richness = "rich"
	RichnessBonanza Richness = "bonanza"
)

// NodeConfig defines yield and reserve parameters for each richness tier.
type NodeConfig struct {
	MaxReserves  int
	RegenPerTick int
	YieldMin     int
	YieldMax     int
	XPReward     int
}

// Configs maps richness tiers to their parameters.
var Configs = map[Richness]NodeConfig{
	RichnessPoor:    {MaxReserves: 200, RegenPerTick: 2, YieldMin: 5, YieldMax: 12, XPReward: 4},
	RichnessNormal:  {MaxReserves: 500, RegenPerTick: 5, YieldMin: 10, YieldMax: 25, XPReward: 6},
	RichnessRich:    {MaxReserves: 1000, RegenPerTick: 8, YieldMin: 20, YieldMax: 50, XPReward: 10},
	RichnessBonanza: {MaxReserves: 300, RegenPerTick: 1, YieldMin: 40, YieldMax: 100, XPReward: 18},
}

// SystemRichness maps system_id → resource_type → Richness tier, used at seed time.
var SystemRichness = map[string]map[string]Richness{
	// Milky Way
	"abydos":   {"naquadah": RichnessRich},
	"chulak":   {"trinium": RichnessNormal},
	"dakara":   {"naquadah": RichnessNormal, "ancient_tech": RichnessBonanza},
	"tollana":  {"trinium": RichnessRich},
	"edora":    {"naquadah": RichnessNormal},
	"viz_ka":   {"naquadriah": RichnessRich},
	"p3x_888":  {"naquadah": RichnessNormal, "trinium": RichnessNormal},
	"orban":    {"trinium": RichnessNormal},
	"vyus":     {"naquadah": RichnessPoor},
	"hebridan": {"trinium": RichnessNormal, "naquadah": RichnessPoor},
	"camelot":  {"ancient_tech": RichnessRich},
	"kallana":  {"naquadah": RichnessNormal},
	"vis_uban": {"ancient_tech": RichnessBonanza},
	// Pegasus
	"lantea":   {"ancient_tech": RichnessRich},
	"sateda":   {"trinium": RichnessNormal},
	"hoff":     {"naquadah": RichnessPoor},
	"m7g_677":  {"ancient_tech": RichnessNormal},
	"olesia":   {"trinium": RichnessNormal},
	"taranis":  {"naquadah": RichnessNormal},
	"doranda":  {"ancient_tech": RichnessBonanza, "naquadah": RichnessRich},
	"genia":    {"naquadriah": RichnessRich},
	"dagan":    {"ancient_tech": RichnessNormal},
	"manaria":  {"naquadah": RichnessNormal, "trinium": RichnessNormal},
	// Destiny's Path
	"novus":           {"naquadah": RichnessPoor},
	"eden":            {"trinium": RichnessNormal, "ancient_tech": RichnessNormal},
	"gravel_pit":      {"naquadah": RichnessNormal},
	"ice_world":       {"trinium": RichnessRich},
	"jungle_world":    {"naquadah": RichnessNormal, "trinium": RichnessNormal},
	"desert_ruins":    {"ancient_tech": RichnessRich},
	"unnamed_outpost": {"ancient_tech": RichnessNormal},
	"crashed_city":    {"naquadah": RichnessNormal, "ancient_tech": RichnessRich},
}

// CargoCapacityByClass defines base cargo capacity for each ship class.
var CargoCapacityByClass = map[string]int{
	"gate_runner_mk1": 80,
	"patrol_craft":    120,
	"destroyer":       200,
	"battlecruiser":   300,
	"mining_barge":    600,
	"freighter":       1200,
}
