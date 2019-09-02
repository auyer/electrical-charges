package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"strconv"

	"golang.org/x/image/font"

	"github.com/auyer/electrical-charges/sprites"
	"github.com/hajimehoshi/ebiten/text"

	"github.com/golang/freetype/truetype"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/inpututil"
)

// distance calculates the distance (Pythagorean Theorem) using the spacial coordinates of the charges
func distance(particle1, particle2 *Sprite) float64 {
	deltaX := float64(particle1.x - particle2.x)
	deltaY := float64(particle1.y - particle2.y)
	return math.Sqrt(deltaX*deltaX+deltaY*deltaY) / 100 // this division turns the distance scale to cm/px instead for m/px
}

// force calculates the force between two charges
func force(particle1 *Sprite, particle2 *Sprite) float64 {
	d := distance(particle1, particle2)
	return k * (particle1.charge * particle2.charge) / (d * d)
}

// field calculates the eletric field on a given radius
func field(charge float64, radius float64) float64 {
	return k * charge/ (radius * radius)
}

// angle calculates the angle in rads by the arc tangent of the tangent formed by the two charges
func angle(particle1 *Sprite, particle2 *Sprite) float64 {
	return math.Atan2(float64(particle1.y-particle2.y), float64(particle1.x-particle2.x))
}

func midPoint(particle1, particle2 *Sprite) (int, int) {
	return (particle1.x + particle2.x) / 2, (particle1.y + particle2.y) / 2
}

// drawElectricalInformation draws the electrical information generated between two charges
func drawElectricalInformation(screen *ebiten.Image, sprite1, sprite2 *Sprite) {
	opt := &ebiten.DrawImageOptions{}
	opt.GeoM.Scale(1, distance(sprite1, sprite2)*100)
	opt.GeoM.Rotate(angle(sprite1, sprite2) + math.Pi/2)
	opt.GeoM.Translate(float64(sprite1.x)+20, float64(sprite1.y)+20)
	screen.DrawImage(line, opt)
	midx, midy := midPoint(sprite1, sprite2)
	text.Draw(screen, fmt.Sprintf("%.2f m", distance(sprite1, sprite2)), theGame.Font, midx, midy, color.White)
	text.Draw(screen, fmt.Sprintf("F= %.2e N", force(sprite1, sprite2)), theGame.Font, sprite2.x, sprite2.y+fontHeight*4, color.White)
	text.Draw(screen, fmt.Sprintf("E= %e N/C", field(sprite1.charge, distance(sprite1, sprite2))), theGame.Font, sprite2.x, sprite2.y+fontHeight/10+fontHeight*5, color.White)
}

var (
	negativeImage, neutralImage, positiveImage *ebiten.Image
	rectangle, line                            *ebiten.Image
	theGame                                    *Game
	fontHeight                                 int
)

const (
	k                = 0.000000009 // Nm²/C²
	fullScreenWidth  = 800
	fullScreenHeight = 600
	screenWidth      = fullScreenWidth
	screenHeight     = fullScreenHeight * .9
)

// Sprite represents an image.
type Sprite struct {
	name   string
	image  *ebiten.Image
	x      int
	y      int
	charge float64
	chosen bool
}

// In returns true if (x, y) is in the sprite, and false otherwise.
func (s *Sprite) In(x, y int) bool {
	// Check the actual color (alpha) value at the specified position
	// so that the result of In becomes natural to users.
	//
	// Note that this is not a good manner to use At for logic
	// since color from At might include some errors on some machines.
	// As this is not so important logic, it's ok to use it so far.
	return s.image.At(x-s.x, y-s.y).(color.RGBA).A > 0
}

// MoveBy moves the sprite by (x, y).
func (s *Sprite) MoveBy(x, y int) {
	w, h := s.image.Size()

	s.x += x
	s.y += y
	if s.x < 0 {
		s.x = 0
	}
	if s.x > screenWidth-w {
		s.x = screenWidth - w
	}
	if s.y < 0 {
		s.y = 0
	}
	if s.y > screenHeight-h {
		s.y = screenHeight - h
	}
}

// Draw draws the sprite.
func (s *Sprite) Draw(screen *ebiten.Image, dx, dy int, alpha float64) {
	op := &ebiten.DrawImageOptions{}
	// op.GeoM.Scale(0.5, 0.5)
	op.GeoM.Translate(float64(s.x+dx), float64(s.y+dy))
	if s.chosen {
		op.ColorM.Scale(0.5, 0.5, 0.5, alpha)
	} else {
		op.ColorM.Scale(1, 1, 1, alpha)
	}
	text.Draw(screen, s.name, theGame.Font, s.x, s.y, color.White)
	screen.DrawImage(s.image, op)

}

// DrawStatistics draws the sprites charge on the top of the screen.
func (s *Sprite) DrawStatistics(screen *ebiten.Image, x, y int, alpha float64) {
	text.Draw(screen, fmt.Sprintf("%s Charge : %.2f C", s.name, s.charge), theGame.Font, x, y, color.White)
}

// StrokeSource represents a input device to provide strokes.
type StrokeSource interface {
	Position() (int, int)
	IsJustReleased() bool
}

// MouseStrokeSource is a StrokeSource implementation of mouse.
type MouseStrokeSource struct{}

// Position returns the cursor position
func (m *MouseStrokeSource) Position() (int, int) {
	return ebiten.CursorPosition()
}

// IsJustReleased checks if the mouse button was released
func (m *MouseStrokeSource) IsJustReleased() bool {
	return inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}

// TouchStrokeSource is a StrokeSource implementation of touch.
type TouchStrokeSource struct {
	ID int
}

// Position returns the touch screen position
func (t *TouchStrokeSource) Position() (int, int) {
	return ebiten.TouchPosition(t.ID)
}

// IsJustReleased checks if the touch command was released
func (t *TouchStrokeSource) IsJustReleased() bool {
	return inpututil.IsTouchJustReleased(t.ID)
}

// Stroke manages the current drag state by mouse.
type Stroke struct {
	source StrokeSource

	// initX and initY represents the position when dragging starts.
	initX int
	initY int

	// currentX and currentY represents the current position
	currentX int
	currentY int

	released bool

	// draggingObject represents a object (sprite in this case)
	// that is being dragged.
	draggingObject interface{}
}

// NewStroke creates a new stroke
func NewStroke(source StrokeSource) *Stroke {
	cx, cy := source.Position()
	return &Stroke{
		source:   source,
		initX:    cx,
		initY:    cy,
		currentX: cx,
		currentY: cy,
	}
}

// Update function updates the stroke information for each game tick
func (s *Stroke) Update() {
	if s.released {
		return
	}
	if s.source.IsJustReleased() {
		s.released = true
		return
	}
	x, y := s.source.Position()
	s.currentX = x
	s.currentY = y
}

// IsReleased checks if the stroke was releases
func (s *Stroke) IsReleased() bool {
	return s.released
}

// Position returns the current stroke position
func (s *Stroke) Position() (int, int) {
	return s.currentX, s.currentY
}

// PositionDiff points the difference between the initial position and the current position
func (s *Stroke) PositionDiff() (int, int) {
	dx := s.currentX - s.initX
	dy := s.currentY - s.initY
	return dx, dy
}

// DraggingObject returns the object being dragged
func (s *Stroke) DraggingObject() interface{} {
	return s.draggingObject
}

// SetDraggingObject sets an object as being dragged
func (s *Stroke) SetDraggingObject(object interface{}) {
	s.draggingObject = object
}

// Game struct stores the game state, its sprites, strokes, Font and the selected sprite
type Game struct {
	strokes      map[*Stroke]struct{}
	sprites      []*Sprite
	Font         font.Face
	ChosenSprite *Sprite
}

func init() {
	rand.Seed(25) // Deterministic rand seed

	// creating a white rectangle to be used in the bottom of the screen
	rectangle, _ = ebiten.NewImage(screenWidth, screenHeight/10, ebiten.FilterNearest)
	rectangle.Fill(color.White)

	// creating the line to link particles
	line, _ = ebiten.NewImage(1, 1, ebiten.FilterNearest)
	line.Fill(color.NRGBA{0x00, 0xff, 0x00, 0xff})

	// negative sprite image
	negimg, _, err := image.Decode(bytes.NewReader(sprites.Negative))
	if err != nil {
		log.Fatal(err)
	}
	negativeImage, _ = ebiten.NewImageFromImage(negimg, ebiten.FilterDefault)

	// neutral sprite image
	netimg, _, err := image.Decode(bytes.NewReader(sprites.Neutral))
	if err != nil {
		log.Fatal(err)
	}
	neutralImage, _ = ebiten.NewImageFromImage(netimg, ebiten.FilterDefault)

	// positive sprite image
	posimg, _, err := image.Decode(bytes.NewReader(sprites.Positive))
	if err != nil {
		log.Fatal(err)
	}
	positiveImage, _ = ebiten.NewImageFromImage(posimg, ebiten.FilterDefault)

	// creating the font
	tt, err := truetype.Parse(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the sprites.
	sprites := []*Sprite{}
	w, h := neutralImage.Size()
	for i := 0; i < 2; i++ {
		s := &Sprite{
			name:   "Q" + strconv.Itoa(i),
			image:  neutralImage,
			x:      rand.Intn(screenWidth - w),
			y:      rand.Intn(screenHeight - h),
			charge: 0,
		}
		sprites = append(sprites, s)
	}

	// Initialize the game.
	theGame = &Game{
		strokes: map[*Stroke]struct{}{},
		sprites: sprites,
		Font: truetype.NewFace(tt, &truetype.Options{
			Size:    12,
			DPI:     142,
			Hinting: font.HintingFull,
		}),
		ChosenSprite: nil,
	}
	b, _, _ := theGame.Font.GlyphBounds('M')
	fontHeight = (b.Max.Y - b.Min.Y).Ceil()
}

// spriteAt function returns a sprite at the requested function or nil if none is found
func (g *Game) spriteAt(x, y int) *Sprite {
	// As the sprites are ordered from back to front,
	// search the clicked/touched sprite in reverse order.
	for i := len(g.sprites) - 1; i >= 0; i-- {
		s := g.sprites[i]
		if s.In(x, y) {
			return s
		}
	}
	return nil
}

// drawHelp draws the help text on the bottom of the screen
func drawHelp(screen *ebiten.Image) {
	textHeight := int(fullScreenHeight - fullScreenHeight*.05)
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(0, fullScreenHeight*.9+fullScreenHeight*.01)
	screen.DrawImage(rectangle, opts)
	text.Draw(screen, "LMB to select charge, drag to move, 'A' to add a new charge, ", theGame.Font, 0, textHeight, color.NRGBA{0xff, 0x00, 0x00, 0xff})
	text.Draw(screen, "'P' to increase charge, 'N' to decrease charge. ", theGame.Font, 0, textHeight+fontHeight+fontHeight/5, color.NRGBA{0xff, 0x00, 0x00, 0xff})
}

func (g *Game) updateStroke(stroke *Stroke) {
	stroke.Update()
	if !stroke.IsReleased() {
		return
	}

	s := stroke.DraggingObject().(*Sprite)
	if s == nil {
		return
	}

	s.MoveBy(stroke.PositionDiff())

	index := -1
	for i, ss := range g.sprites {
		if ss == s {
			index = i
			break
		}
	}

	// Move the dragged sprite to the front.
	g.sprites = append(g.sprites[:index], g.sprites[index+1:]...)
	g.sprites = append(g.sprites, s)

	stroke.SetDraggingObject(nil)
}

func (g *Game) update(screen *ebiten.Image) error {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s := NewStroke(&MouseStrokeSource{})
		spriteAtPos := g.spriteAt(s.Position())
		s.SetDraggingObject(spriteAtPos)
		g.strokes[s] = struct{}{}
		for _, s := range g.sprites {
			s.chosen = false
		}
		if spriteAtPos != nil {
			spriteAtPos.chosen = true
		}
		g.ChosenSprite = spriteAtPos
	}
	for _, id := range inpututil.JustPressedTouchIDs() {
		s := NewStroke(&TouchStrokeSource{id})
		spriteAtPos := g.spriteAt(s.Position())
		s.SetDraggingObject(spriteAtPos)
		g.strokes[s] = struct{}{}
		for _, s := range g.sprites {
			s.chosen = false
		}
		if spriteAtPos != nil {
			spriteAtPos.chosen = true
		}
		g.ChosenSprite = spriteAtPos
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		s := &Sprite{
			name:   "Q" + strconv.Itoa(len(theGame.sprites)),
			image:  neutralImage,
			x:      rand.Intn(screenWidth),
			y:      rand.Intn(screenHeight),
			charge: 0.,
		}
		theGame.sprites = append(theGame.sprites, s)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		for _, s := range g.sprites {
			if s.chosen {
				s.charge += 0.1
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		for _, s := range g.sprites {
			if s.chosen {
				s.charge -= 0.1
			}
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		for _, s := range g.sprites {
			if s.chosen {
				s.y -= screenHeight / 10
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		for _, s := range g.sprites {
			if s.chosen {
				s.y += screenHeight / 10
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		for _, s := range g.sprites {
			if s.chosen {
				s.x += screenWidth / 10
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		for _, s := range g.sprites {
			if s.chosen {
				s.x -= screenWidth / 10
			}
		}
	}

	for s := range g.strokes {
		g.updateStroke(s)
		if s.IsReleased() {
			delete(g.strokes, s)
		}
	}

	if ebiten.IsDrawingSkipped() {
		return nil
	}

	drawHelp(screen)
	draggingSprites := map[*Sprite]struct{}{}
	for s := range g.strokes {
		if sprite := s.DraggingObject().(*Sprite); sprite != nil {
			draggingSprites[sprite] = struct{}{}
		}
	}

	for _, s := range g.sprites {
		if _, ok := draggingSprites[s]; ok {
			continue
		}
		switch {
		case s.charge > 0.:
			s.image = positiveImage
		case s.charge < 0.:
			s.image = negativeImage
		default:
			s.image = neutralImage
		}
		s.Draw(screen, 0, 0, 1)

		if s.chosen {
			s.DrawStatistics(screen, screenWidth*.1, screenHeight*.1, 1)
		}
		if g.ChosenSprite != nil && g.ChosenSprite != s {
			drawElectricalInformation(screen, g.ChosenSprite, s)
		}
	}
	for s := range g.strokes {
		dx, dy := s.PositionDiff()
		if sprite := s.DraggingObject().(*Sprite); sprite != nil {
			sprite.Draw(screen, dx, dy, 0.5)
		}
	}
	return nil
}

func main() {
	if err := ebiten.Run(theGame.update, fullScreenWidth, fullScreenHeight, 1, "Electrical Charges demonstration"); err != nil {
		log.Fatal(err)
	}
}
