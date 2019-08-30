// Copyright 2018 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build example jsgo

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math/rand"
	"strconv"
	"time"

	"golang.org/x/image/font"

	"github.com/auyer/electrical-charges/sprites"
	"github.com/hajimehoshi/ebiten/text"

	"github.com/golang/freetype/truetype"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/inpututil"
)

var (
	negativeImage, neutralImage, positiveImage *ebiten.Image
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	FullScreenWidth  = 400
	FullScreenHeight = 300
	screenWidth      = FullScreenWidth
	screenHeight     = FullScreenHeight * .9
)

type Font struct {
	Face        font.Face
	FontMHeight int
}

type relationForce struct {
	name  string
	force float32
}

// Sprite represents an image.
type Sprite struct {
	name   string
	image  *ebiten.Image
	x      int
	y      int
	charge float32
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
	op.GeoM.Translate(float64(s.x+dx), float64(s.y+dy))
	if s.chosen {
		op.ColorM.Scale(2, 2, 2, alpha)
		// screen.DrawImage()
	} else {
		op.ColorM.Scale(1, 1, 1, alpha)
	}
	text.Draw(screen, s.name, theGame.Font, s.x, s.y, color.White)
	screen.DrawImage(s.image, op)
	screen.DrawImage(s.image, op)

}

// Draw draws the sprite.
func (s *Sprite) DrawStatistics(screen *ebiten.Image, x, y int, alpha float64) {
	text.Draw(screen, fmt.Sprintf("%s Charge : %f", s.name, s.charge), theGame.Font, x, y, color.White)
}

// StrokeSource represents a input device to provide strokes.
type StrokeSource interface {
	Position() (int, int)
	IsJustReleased() bool
}

// MouseStrokeSource is a StrokeSource implementation of mouse.
type MouseStrokeSource struct{}

func (m *MouseStrokeSource) Position() (int, int) {
	return ebiten.CursorPosition()
}

func (m *MouseStrokeSource) IsJustReleased() bool {
	return inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft)
}

// TouchStrokeSource is a StrokeSource implementation of touch.
type TouchStrokeSource struct {
	ID int
}

func (t *TouchStrokeSource) Position() (int, int) {
	return ebiten.TouchPosition(t.ID)
}

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

func (s *Stroke) IsReleased() bool {
	return s.released
}

func (s *Stroke) Position() (int, int) {
	return s.currentX, s.currentY
}

func (s *Stroke) PositionDiff() (int, int) {
	dx := s.currentX - s.initX
	dy := s.currentY - s.initY
	return dx, dy
}

func (s *Stroke) DraggingObject() interface{} {
	return s.draggingObject
}

func (s *Stroke) SetDraggingObject(object interface{}) {
	s.draggingObject = object
}

type Game struct {
	strokes      map[*Stroke]struct{}
	sprites      []*Sprite
	Font         font.Face
	ChosenSprite *Sprite
}

var theGame *Game

var rectangle, line *ebiten.Image

func init() {
	// Decode image from a byte slice instead of a file so that
	// this example works in any working directory.
	// If you want to use a file, there are some options:
	// 1) Use os.Open and pass the file to the image decoder.
	//    This is a very regular way, but doesn't work on browsers.
	// 2) Use ebitenutil.OpenFile and pass the file to the image decoder.
	//    This works even on browsers.
	// 3) Use ebitenutil.NewImageFromFile to create an ebiten.Image directly from a file.
	//    This also works on browsers.
	rectangle, _ = ebiten.NewImage(screenWidth, screenHeight/10, ebiten.FilterNearest)
	rectangle.Fill(color.White)

	line, _ = ebiten.NewImage(5, 1, ebiten.FilterNearest)
	line.Fill(color.NRGBA{0xff, 0x00, 0x00, 0xff})

	negimg, _, err := image.Decode(bytes.NewReader(sprites.Negative))
	if err != nil {
		log.Fatal(err)
	}
	netimg, _, err := image.Decode(bytes.NewReader(sprites.Neutral))
	if err != nil {
		log.Fatal(err)
	}
	posimg, _, err := image.Decode(bytes.NewReader(sprites.Positive))
	if err != nil {
		log.Fatal(err)
	}
	negativeImage, _ = ebiten.NewImageFromImage(negimg, ebiten.FilterDefault)
	neutralImage, _ = ebiten.NewImageFromImage(netimg, ebiten.FilterDefault)
	positiveImage, _ = ebiten.NewImageFromImage(posimg, ebiten.FilterDefault)

	// Initialize the sprites.
	tt, err := truetype.Parse(goregular.TTF)
	if err != nil {
		log.Fatal(err)
	}
	uiFont := &Font{Face: truetype.NewFace(tt, &truetype.Options{
		Size:    6,
		DPI:     172,
		Hinting: font.HintingFull,
	})}
	b, _, _ := uiFont.Face.GlyphBounds('M')
	uiFont.FontMHeight = (b.Max.Y - b.Min.Y).Ceil()

	sprites := []*Sprite{}
	w, h := neutralImage.Size()
	for i := 0; i < 2; i++ {
		s := &Sprite{
			name:   "Q" + strconv.Itoa(i),
			image:  neutralImage,
			x:      rand.Intn(screenWidth - w),
			y:      rand.Intn(screenHeight - h),
			charge: rand.Float32(),
		}
		sprites = append(sprites, s)
	}

	// Initialize the game.
	theGame = &Game{
		strokes:      map[*Stroke]struct{}{},
		sprites:      sprites,
		Font:         uiFont.Face,
		ChosenSprite: nil,
	}
}

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

func drawHelp(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(0, FullScreenHeight*.9+FullScreenHeight*.01)
	screen.DrawImage(rectangle, opts)
	text.Draw(screen, "LMB to select charge, drag to move, '9' to increase charge, ", theGame.Font, 0, FullScreenHeight-FullScreenHeight*.05, color.NRGBA{0xff, 0x00, 0x00, 0xff})
	text.Draw(screen, "'0' to decrease charge, 'A' to add a new charge.", theGame.Font, 0, FullScreenHeight-FullScreenHeight*.01, color.NRGBA{0xff, 0x00, 0x00, 0xff})
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
			name:   "Q" + strconv.Itoa(len(theGame.sprites)+1),
			image:  neutralImage,
			x:      rand.Intn(screenWidth),
			y:      rand.Intn(screenHeight),
			charge: 0.,
		}
		theGame.sprites = append(theGame.sprites, s)
	}

	if inpututil.IsKeyJustPressed(ebiten.Key0) {
		for _, s := range g.sprites {
			if s.chosen {
				s.charge += 0.1
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.Key9) {
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
		if theGame.ChosenSprite != nil {
			opt := &ebiten.DrawImageOptions{}
			opt.GeoM.Translate(float64(s.x), float64(s.y))
			// opt.GeoM.Rotate()
			screen.DrawImage(line, opt)
		}
	}
	for s := range g.strokes {
		dx, dy := s.PositionDiff()
		if sprite := s.DraggingObject().(*Sprite); sprite != nil {
			sprite.Draw(screen, dx, dy, 0.5)
		}
	}

	// ebitenutil.DebugPrint(screen, "Drag & Drop the carges!")

	return nil
}

func main() {
	if err := ebiten.Run(theGame.update, FullScreenWidth, FullScreenHeight, 2, "Drag & Drop (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}
