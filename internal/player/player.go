package player

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
)

type Player struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	playing bool
}

func New() *Player {
	return &Player{}
}

func (p *Player) Play(filePath string, volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playing {
		return nil
	}

	// --gain is a float multiplier: 1.0 = normal, 0.1 = 10%, etc.
	vol := float64(volume) / 100.0
	volStr := fmt.Sprintf("%.2f", vol)

	p.cmd = exec.Command("cvlc", "--play-and-exit", "--aout=pulse", "--gain", volStr, "--loop", filePath)
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("error starting player: %w", err)
	}

	p.playing = true
	log.Printf("Playing: %s (volume: %d%%)", filePath, volume)

	go func() {
		p.cmd.Wait()
		p.mu.Lock()
		p.playing = false
		p.mu.Unlock()
	}()

	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.playing || p.cmd == nil || p.cmd.Process == nil {
		return
	}

	if err := p.cmd.Process.Kill(); err != nil {
		log.Printf("Error stopping player: %v", err)
	}
	p.playing = false
	log.Println("Playback stopped")
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}
