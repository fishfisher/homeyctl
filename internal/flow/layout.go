package flow

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Card geometry in Homey canvas units. Homey does not store rendered card
// sizes, so these are estimates calibrated against the Advanced Flow editor:
// regular cards render about 340 units wide, and hand-built flows on a real
// Homey place connected cards a median of 420 apart horizontally.
const (
	cardWidth      = 340.0
	joinWidth      = 90.0 // any / all
	joinHeight     = 50.0
	pillWidth      = 140.0 // start / delay render as compact pills
	pillHeight     = 50.0
	charsPerLine   = 30.0
	lineHeight     = 25.0
	cardPadding    = 24.0
	titleChars     = 14.0 // the card's own wording, which is not in the document
	tokenChars     = 16.0 // a [[tag]] renders as a chip roughly this wide
	noteLineHeight = 22.0

	// LayoutColumnPitch is the horizontal distance between layout columns.
	LayoutColumnPitch = 420.0
	// LayoutGap is the minimum vertical space between stacked cards.
	LayoutGap = 40.0
	// narrowNoteWidth is the widest note that is stacked above its card; wider
	// notes span columns and go in a band above the graph instead.
	narrowNoteWidth = 400.0
	layoutOrigin    = 60.0
	// How deep two estimated rectangles must intersect before validation warns.
	// Card width is fixed in the editor, so horizontal overlap is judged
	// tightly; height depends on how text wraps, so vertical overlap gets more
	// slack. Layout spaces by the full estimate either way.
	overlapSlackX = 8.0
	overlapSlackY = 20.0
)

var tagPattern = regexp.MustCompile(`\[\[[^\]]*\]\]`)

// Rect is an estimated card rectangle.
type Rect struct {
	X, Y, W, H float64
}

func (r Rect) overlaps(o Rect) bool {
	return r.X+overlapSlackX < o.X+o.W && o.X+overlapSlackX < r.X+r.W &&
		r.Y+overlapSlackY < o.Y+o.H && o.Y+overlapSlackY < r.Y+r.H
}

// EstimateSize returns the approximate rendered width and height of a card.
func EstimateSize(card map[string]any) (float64, float64) {
	cardType, _ := card["type"].(string)
	switch cardType {
	case "any", "all":
		return joinWidth, joinHeight
	case "note":
		w, ok := numeric(card["width"])
		if !ok || w <= 0 {
			w = cardWidth
		}
		if h, ok := numeric(card["height"]); ok && h > 0 {
			return w, h
		}
		text, _ := card["value"].(string)
		perLine := math.Max(10, (w-30)/9)
		lines := 0.0
		for _, line := range strings.Split(text, "\n") {
			lines += math.Max(1, math.Ceil(float64(len([]rune(line)))/perLine))
		}
		return w, cardPadding + lines*noteLineHeight
	case "start", "delay":
		return pillWidth, pillHeight
	}
	chars := titleChars + argChars(card["args"])
	// One line for the app/device label above the card text.
	lines := 1 + math.Max(1, math.Ceil(chars/charsPerLine))
	return cardWidth, cardPadding + lines*lineHeight
}

// argChars approximates how much text a card's arguments add when rendered.
func argChars(raw any) float64 {
	switch value := raw.(type) {
	case map[string]any:
		if name, ok := value["name"].(string); ok && name != "" {
			return float64(len([]rune(name)))
		}
		total := 0.0
		for _, v := range value {
			total += argChars(v)
		}
		return total
	case []any:
		total := 0.0
		for _, v := range value {
			total += argChars(v)
		}
		return total
	case string:
		tags := tagPattern.FindAllString(value, -1)
		plain := tagPattern.ReplaceAllString(value, "")
		return float64(len([]rune(plain))) + float64(len(tags))*tokenChars
	case nil:
		return 0
	default:
		return 4
	}
}

func cardRect(card map[string]any) (Rect, bool) {
	x, okX := numeric(card["x"])
	y, okY := numeric(card["y"])
	if !okX || !okY {
		return Rect{}, false
	}
	w, h := EstimateSize(card)
	return Rect{X: x, Y: y, W: w, H: h}, true
}

// validateOverlaps warns about cards whose estimated rectangles intersect.
// Sizes are estimates, so this is a warning and never an error.
func validateOverlaps(cards map[string]any, report *Report) {
	ids := make([]string, 0, len(cards))
	rects := map[string]Rect{}
	for id, raw := range cards {
		card, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if rect, ok := cardRect(card); ok {
			ids = append(ids, id)
			rects[id] = rect
		}
	}
	sort.Strings(ids)
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			if rects[a].overlaps(rects[b]) {
				report.addWarning("card_overlap", "cards."+a,
					"card appears to overlap card "+b+" in the editor (estimated sizes); run `homeyctl flows layout` to rearrange")
			}
		}
	}
}

// LayoutResult summarizes what Layout did.
type LayoutResult struct {
	Cards    int `json:"cards"`
	Notes    int `json:"notes"`
	Columns  int `json:"columns"`
	Overlaps int `json:"overlapsRemaining"`
}

type layoutNode struct {
	id      string
	card    map[string]any
	origX   float64
	origY   float64
	w, h    float64
	rank    int
	desired float64 // desired center y
	order   float64 // tie-break within a column
	notes   []*layoutNode
	isNote  bool
	placedY float64
}

// Layout rewrites x and y of every card in an advanced flow document in
// place: columns follow execution order left to right, chains stay level,
// branches stack in port order (true, false, error), and each note stays
// with the card it was placed nearest to. Nothing else is changed.
func Layout(document map[string]any) LayoutResult {
	cards, ok := objectAt(document, "cards")
	if !ok {
		return LayoutResult{}
	}

	nodes := map[string]*layoutNode{}
	var notes []*layoutNode
	for id, raw := range cards {
		card, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		x, _ := numeric(card["x"])
		y, _ := numeric(card["y"])
		w, h := EstimateSize(card)
		n := &layoutNode{id: id, card: card, origX: x, origY: y, w: w, h: h}
		if card["type"] == "note" {
			n.isNote = true
			notes = append(notes, n)
			continue
		}
		nodes[id] = n
	}
	if len(nodes) == 0 {
		return LayoutResult{Notes: len(notes)}
	}

	// Edges in port order, so a condition's true branch sorts above its
	// false branch and both above the error branch.
	type edge struct {
		to   string
		port int
	}
	out := map[string][]edge{}
	indegree := map[string]int{}
	for id, n := range nodes {
		for port, field := range outputFields {
			targets, _ := stringArray(n.card[field])
			for _, target := range targets {
				if _, ok := nodes[target]; ok {
					out[id] = append(out[id], edge{to: target, port: portOrder(field, port)})
					indegree[target]++
				}
			}
		}
	}

	// Depth-first from entry cards in the author's top-to-bottom order;
	// edges back into the current path are loop edges and do not rank.
	sorted := sortedNodes(nodes)
	roots := make([]*layoutNode, 0)
	for _, n := range sorted {
		if indegree[n.id] == 0 {
			roots = append(roots, n)
		}
	}
	state := map[string]int{} // 0 new, 1 on path, 2 done
	var topo []string
	backEdge := map[[2]string]bool{}
	var visit func(id string)
	visit = func(id string) {
		state[id] = 1
		for _, e := range out[id] {
			switch state[e.to] {
			case 0:
				visit(e.to)
			case 1:
				backEdge[[2]string{id, e.to}] = true
			}
		}
		state[id] = 2
		topo = append(topo, id)
	}
	for _, r := range roots {
		if state[r.id] == 0 {
			visit(r.id)
		}
	}
	// Cards only reachable through a cycle.
	for _, n := range sorted {
		if state[n.id] == 0 {
			visit(n.id)
		}
	}
	for i, j := 0, len(topo)-1; i < j; i, j = i+1, j-1 {
		topo[i], topo[j] = topo[j], topo[i]
	}

	preds := map[string][]string{}
	maxRank := 0
	for _, id := range topo {
		for _, e := range out[id] {
			if backEdge[[2]string{id, e.to}] {
				continue
			}
			preds[e.to] = append(preds[e.to], id)
			if r := nodes[id].rank + 1; r > nodes[e.to].rank {
				nodes[e.to].rank = r
			}
		}
	}
	for _, n := range nodes {
		if n.rank > maxRank {
			maxRank = n.rank
		}
	}

	// Attach each note to the card nearest to it in the original placement.
	var wideNotes []*layoutNode
	for _, note := range notes {
		anchor := nearestCard(note, sorted)
		if anchor == nil {
			continue
		}
		if note.w <= narrowNoteWidth {
			anchor.notes = append(anchor.notes, note)
		} else {
			note.rank = anchor.rank
			note.order = anchor.origY
			wideNotes = append(wideNotes, note)
		}
	}

	columns := make([][]*layoutNode, maxRank+1)
	for _, n := range sorted {
		columns[n.rank] = append(columns[n.rank], n)
	}

	// Entry cards keep the author's vertical order; everything else aims for
	// the mean center of the cards feeding it.
	nextSource := layoutOrigin
	graphTop := math.Inf(1)
	for rank, column := range columns {
		for _, n := range column {
			if len(preds[n.id]) == 0 {
				continue
			}
			sum, port := 0.0, 0.0
			for _, p := range preds[n.id] {
				pn := nodes[p]
				sum += pn.placedY + pn.h/2
				for _, e := range out[p] {
					if e.to == n.id {
						port = math.Max(port, float64(e.port))
					}
				}
			}
			n.desired = sum / float64(len(preds[n.id]))
			n.order = port
		}
		sort.SliceStable(column, func(i, j int) bool {
			a, b := column[i], column[j]
			if len(preds[a.id]) == 0 && len(preds[b.id]) == 0 {
				return a.origY < b.origY
			}
			if a.desired != b.desired {
				return a.desired < b.desired
			}
			if a.order != b.order {
				return a.order < b.order
			}
			return a.origY < b.origY
		})

		x := layoutOrigin + float64(rank)*LayoutColumnPitch
		bottom := math.Inf(-1)
		for _, n := range column {
			top := n.desired - n.h/2
			if len(preds[n.id]) == 0 {
				top = nextSource // entry cards stack in the author's order
			}
			for _, note := range n.notes {
				noteTop := math.Max(top-note.h-LayoutGap, bottom+LayoutGap)
				if math.IsInf(bottom, -1) {
					noteTop = top - note.h - LayoutGap
				}
				note.placedY = noteTop
				setXY(note.card, x, noteTop)
				graphTop = math.Min(graphTop, noteTop)
				bottom = noteTop + note.h
				top = math.Max(top, bottom+LayoutGap)
			}
			if !math.IsInf(bottom, -1) {
				top = math.Max(top, bottom+LayoutGap)
			}
			n.placedY = top
			setXY(n.card, x, top)
			graphTop = math.Min(graphTop, top)
			bottom = top + n.h
			if len(preds[n.id]) == 0 {
				nextSource = bottom + LayoutGap*2
			}
		}
	}

	// Wide notes go in a band above the graph, left to right by anchor column.
	if len(wideNotes) > 0 {
		tallest := 0.0
		for _, n := range wideNotes {
			tallest = math.Max(tallest, n.h)
		}
		sort.SliceStable(wideNotes, func(i, j int) bool {
			if wideNotes[i].rank != wideNotes[j].rank {
				return wideNotes[i].rank < wideNotes[j].rank
			}
			return wideNotes[i].order < wideNotes[j].order
		})
		right := math.Inf(-1)
		for _, n := range wideNotes {
			x := math.Max(layoutOrigin+float64(n.rank)*LayoutColumnPitch, right+LayoutGap)
			setXY(n.card, x, graphTop-tallest-LayoutGap*2)
			right = x + n.w
		}
	}

	normalize(cards)

	result := LayoutResult{Cards: len(nodes), Notes: len(notes), Columns: maxRank + 1}
	report := Report{}
	validateOverlaps(cards, &report)
	result.Overlaps = len(report.Warnings)
	return result
}

// portOrder ranks output ports so true < success < false < error.
func portOrder(field string, fallback int) int {
	switch field {
	case "outputTrue":
		return 0
	case "outputSuccess":
		return 1
	case "outputFalse":
		return 2
	case "outputError":
		return 3
	}
	return fallback
}

func sortedNodes(nodes map[string]*layoutNode) []*layoutNode {
	list := make([]*layoutNode, 0, len(nodes))
	for _, n := range nodes {
		list = append(list, n)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].origY != list[j].origY {
			return list[i].origY < list[j].origY
		}
		if list[i].origX != list[j].origX {
			return list[i].origX < list[j].origX
		}
		return list[i].id < list[j].id
	})
	return list
}

// nearestCard finds the card a note most plausibly describes: the closest
// one, preferring cards below the note since notes usually sit above what
// they explain.
func nearestCard(note *layoutNode, cards []*layoutNode) *layoutNode {
	var best *layoutNode
	bestDist := math.Inf(1)
	nx, ny := note.origX+note.w/2, note.origY+note.h
	for _, c := range cards {
		cx, cy := c.origX+c.w/2, c.origY
		dy := cy - ny
		if dy < 0 {
			dy = -dy * 2 // above the note: less likely to be its subject
		}
		dx := math.Max(0, math.Abs(cx-nx)-note.w/2)
		if d := math.Hypot(dx, dy); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func setXY(card map[string]any, x, y float64) {
	card["x"] = math.Round(x)
	card["y"] = math.Round(y)
}

// normalize shifts the whole graph so its top-left corner is at the origin.
func normalize(cards map[string]any) {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, raw := range cards {
		card, _ := raw.(map[string]any)
		if x, ok := numeric(card["x"]); ok {
			minX = math.Min(minX, x)
		}
		if y, ok := numeric(card["y"]); ok {
			minY = math.Min(minY, y)
		}
	}
	if math.IsInf(minX, 1) {
		return
	}
	for _, raw := range cards {
		card, _ := raw.(map[string]any)
		x, _ := numeric(card["x"])
		y, _ := numeric(card["y"])
		setXY(card, x-minX+layoutOrigin, y-minY+layoutOrigin)
	}
}
