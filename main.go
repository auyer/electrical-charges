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
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"github.com/hajimehoshi/ebiten/inpututil"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	screenWidth  = 320
	screenHeight = 240
)

// Sprite represents an image.
type Sprite struct {
	image *ebiten.Image
	x     int
	y     int
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
	op.ColorM.Scale(1, 1, 1, alpha)
	screen.DrawImage(s.image, op)
	screen.DrawImage(s.image, op)
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
	strokes map[*Stroke]struct{}
	sprites []*Sprite
}

var theGame *Game

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
	img, _, err := image.Decode(bytes.NewReader(negative))
	if err != nil {
		log.Fatal(err)
	}
	ebitenImage, _ := ebiten.NewImageFromImage(img, ebiten.FilterDefault)

	// Initialize the sprites.
	sprites := []*Sprite{}
	w, h := ebitenImage.Size()
	for i := 0; i < 10; i++ {
		s := &Sprite{
			image: ebitenImage,
			x:     rand.Intn(screenWidth - w),
			y:     rand.Intn(screenHeight - h),
		}
		sprites = append(sprites, s)
	}

	// Initialize the game.
	theGame = &Game{
		strokes: map[*Stroke]struct{}{},
		sprites: sprites,
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
		s.SetDraggingObject(g.spriteAt(s.Position()))
		g.strokes[s] = struct{}{}
	}
	for _, id := range inpututil.JustPressedTouchIDs() {
		s := NewStroke(&TouchStrokeSource{id})
		s.SetDraggingObject(g.spriteAt(s.Position()))
		g.strokes[s] = struct{}{}
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
		s.Draw(screen, 0, 0, 1)
	}
	for s := range g.strokes {
		dx, dy := s.PositionDiff()
		if sprite := s.DraggingObject().(*Sprite); sprite != nil {
			sprite.Draw(screen, dx, dy, 0.5)
		}
	}

	ebitenutil.DebugPrint(screen, "Drag & Drop the carges!")

	return nil
}

func main() {
	if err := ebiten.Run(theGame.update, screenWidth, screenHeight, 2, "Drag & Drop (Ebiten Demo)"); err != nil {
		log.Fatal(err)
	}
}

var negative = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x002\x00\x00\x002\b\x06\x00\x00\x00\x1e?\x88\xb1\x00\x00\x00\tpHYs\x00\x00\x0fa\x00\x00\x0fa\x01\xa8?\xa7i\x00\x00\x04KIDAThC\xedYmhMa\x1c\u007fνw\xbb{i\x9b\raI\xc4\a2C^RF\xf3^\xa4P\"\x12\xb5\x92\xa2\x94\xe4\x83$\xa5\xf8\xe4\x13!\x8d(J\xc8\xfb\x87yI\xf3\x1a\x86$$|1\xa3\x99i7{i\xb7\xed\xde\xe3\xf7\xbb\xbb\xb7\x9d\x9d{\x9es\x9fsv\xefv\xd5\xfd\xaf_\xe7\xec\x9e\xff\xdb\xef\xff\xbc\x9c\xe7y\x8e\x10\x19\xc9T S\x01\xbb\nh\xc9.OՓ\xcbtI\xbfc\x81\xe1@\x1e\x10\x04\xfe\x00\xf5\xa7+\xd6v&;f,\xa0#\xbf\xd1D\xadl*\xf0\xe3|`&0\x05\x18o\xa1Ԁ\xdf>\x00\xaf\x81\xa7\xc0] d\xd4\x03QG\xf9Ĕ}\xae\xacz\x8d\xf2q\xbb\x1eX\t,\x01r\x13\xf8\x1b\x8d\xe7Ĳ\xa8^-\xae\xb7\x81K\x00I\xba\x96\xfe\x10Y\x87\xa8\x9b\x80宣\vQ\t[b\rp\x018\t\xe8n\xfc9\x1e#\xe8Z\xac\xe8N\xa0\n\x18\xe2&\xa8\x8d\xcd9<;\n\xbc\x91\xe9Ⱥ^\\\x8b،\x01\xfa\x9e\r\xec\x89V0\xc9\x1c\"\xee6\x03\xe3\x80#\xc0-'\x01\x9ct-\x0e\xe6\xfd\xc0b'\x01\\\xe8r\xc2`K\xe7\x00\x91)PE<*JЙ\n\xec\x1b\x00\x12\xb1t\xcaq\xb3\x17P\x1e\u007f*D\n\xe1p7\xb0T\x91t\xb2\xd4X\xbc]@\x99\x8aC\x15\"\x1c\xd8\x1bU\x9c\xa5@g!|\xeeP\xf1\x9b\x88H%\x9clQq\x94B\x1dN\x00\x84\xad$\"\u0096\xe0,2\x98\xe2G\xf0\r\x00\xbb\xb8T\xec\x88,\x80\xd5\xea\xc1d`\x88\xcdU\x83\xed\xdaŎ\xc8\n\x18\x97\xa4\t\x11\xa6a;\x83Ɉ\x14\xc0pQ\x1a\x91`*\x1c\xf83d9Ɉ̅\x01\xa7\xbft\x92\"$ü,EFdz:10\xe4\"\xcdKFdb\xb2\x88\xe4{\xb2\x92\xe5\x8a~&ɜY\xae~\xb1p|\b\x83yF#\xbf\xe6\x15\xe5\xf9Cń\x9c\"-\xc7\xe3\x13\xac@\x18\xb0[>kx\xaa\x9bV\xe5\xd4\xe7:\x9dW\xda\xfbp\xe7\xf7\xfaD\x96\xe6\x11!=,\xda\xc3ݢ>\xd8*\xeaښ\xf4@77\x96}\xe47\xfe+\xc5\n\xb8\xcff\x8c\x1a\xb2E\xe3\b\xb3\x87\xb2\xbc\x121\xa7`\xa4\x96\a\x12)\x13\x90Ʌ\xffb\xaf\x9fĴ{\x81\xefz\x17\xc8\x19\x84[\xe7b\xa0ٜ\x83\xack\xf1%\xd4G\xc6\xf8\vRK\xc2\x10ͫi\xa24;_H\x8a\xe6\xb5*\xa4\x8cH\x8bY9\x10\n\x8a\xee\xbe\xd5IYð;\xb2[u\xeaq=\x88\a\x17\x1dV\x81e\xfd\x84\xfb\xe7iF\x83\xb7\xed\xcdz\xa17[C\xcb\b\x1f*\xe6A\xdf\x0e#`\xcf8\xe8\x1d+\xb1{ٕ>\x8d\xcfXI\x0f\xfc\xf1/\xac\xeb\x82]\xa9\xb1\xab#2F\x82\xe18\"\r\x18\x1f\xadN\x88|5+\xb7\xa0B5\x81z\x9d\xfd\xb7'p\xfc\xe6\xda\xfc\x9b\x95\x8e\xd9/u\xe8/\x1b\x93I[\xa8+29\xf0ځAo!_d\xdd@\xb6\xd5}oe\xc0\n5\x86-[6e\xdd\xcc\xe4\xf8\x9d,\x90l\x8c\xbc\x84A\xdc8\x19\xa8lm\xe2\xd49%\xc2\x16y\x94\x06\x89\x1bS`k\xf0\xfdf)v\xab_\x9e\x02\xa6\x93\xdcA2<vuL\xe4\x1a,\x9e\xa7\t\x93\x1f\xc8\xe3\x86].v-\xd2\bC\x1ee\xa6\x83\xf0X\xe8\x99[\"\xb4;\x03\\\x1fd&\xaf\x10\xffl\xa2\x1c\x12\xed\xd9\xff\xc2\xc1)\xe0s\"G)zΗ\x1f\xe3K\xa7\xddX\xdcDD\xa8W\x03\x1c\x03Hj\xa0\x85q\xabU\x82\xaa\x10\xa1\x1f:\xe4ylܺZ%\x88K\x9d\xe3\xb0;\xa4j\xebdM~\x10N\xb9L\xe2\xe9_\xb2O\xe1\xcd\xf9\xf2D\xfe\x00ЮJ\xc4\xcdg\x85mp\xbe\x1d\x98\xac\x1aā\x1eg\xca\x13\xc0a\xc0r\xb1\xa5\xfcYA!(?\xc6|\x02\xb6\x02\xfcZ\x95,\xe1\v\xb8\x1a\x89^q\xe3\xd0I\xd72\xfa\xaf\xc5?\x8f\x81\a\x00\x0f\xcex\x80\xe6V^\xc0\xf0*\xc0\x8f<Mn\x9d\xb8%\xc2x\xdc,pF9\x0f\xac\x8a\x92\xe1>\u007f\x82B2?\xa1Ï\xa1\xf7\x81\x9b\xc0/\x05\x1b[\x95\xfe\x10\x899\xe6\xae\xedb\x14\xc3p\x9d\x05p\xfc\xf0\xccx\x14\xc0\xf3\xa8\xb6h\xb2\xdfp\xfd\b\xf0\xabn}\u007f\x93\xcf\xd8g*\x90\xa9@\xa6\x02\xffo\x05\xfe\x01\xe0v\xe7q\xee-\xef\xaf\x00\x00\x00\x00IEND\xaeB`\x82")
