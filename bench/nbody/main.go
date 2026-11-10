package main

import (
	"fmt"
	"math"
)

type body struct {
	x, y, z, vx, vy, vz, mass float64
}

func system() []body {
	const solar = 4 * math.Pi * math.Pi
	const days = 365.24
	bodies := []body{
		{mass: solar},
		{
			x:    4.84143144246472090e+00,
			y:    -1.16032004402742839e+00,
			z:    -1.03622044471123109e-01,
			vx:   1.66007664274403694e-03 * days,
			vy:   7.69901118419740425e-03 * days,
			vz:   -6.90460016972063023e-05 * days,
			mass: 9.54791938424326609e-04 * solar,
		},
		{
			x:    8.34336671824457987e+00,
			y:    4.12479856412430479e+00,
			z:    -4.03523417114321381e-01,
			vx:   -2.76742510726862411e-03 * days,
			vy:   4.99852801234917238e-03 * days,
			vz:   2.30417297573763929e-05 * days,
			mass: 2.85885980666130812e-04 * solar,
		},
		{
			x:    1.28943695621391310e+01,
			y:    -1.51111514016986312e+01,
			z:    -2.23307578892655734e-01,
			vx:   2.96460137564761618e-03 * days,
			vy:   2.37847173959480950e-03 * days,
			vz:   -2.96589568540237556e-05 * days,
			mass: 4.36624404335156298e-05 * solar,
		},
		{
			x:    1.53796971148509165e+01,
			y:    -2.59193146099879641e+01,
			z:    1.79258772950371181e-01,
			vx:   2.68067772490389322e-03 * days,
			vy:   1.62824170038242295e-03 * days,
			vz:   -9.51592254519715870e-05 * days,
			mass: 5.15138902046611451e-05 * solar,
		},
	}
	var px, py, pz float64
	for _, b := range bodies {
		px += b.vx * b.mass
		py += b.vy * b.mass
		pz += b.vz * b.mass
	}
	bodies[0].vx = -px / solar
	bodies[0].vy = -py / solar
	bodies[0].vz = -pz / solar
	return bodies
}

func advance(bodies []body, dt float64) {
	for i := range bodies {
		for j := i + 1; j < len(bodies); j++ {
			dx := bodies[i].x - bodies[j].x
			dy := bodies[i].y - bodies[j].y
			dz := bodies[i].z - bodies[j].z
			d2 := dx*dx + dy*dy + dz*dz
			mag := dt / (d2 * math.Sqrt(d2))
			mi := bodies[i].mass * mag
			mj := bodies[j].mass * mag
			bodies[i].vx -= dx * mj
			bodies[i].vy -= dy * mj
			bodies[i].vz -= dz * mj
			bodies[j].vx += dx * mi
			bodies[j].vy += dy * mi
			bodies[j].vz += dz * mi
		}
	}
	for i := range bodies {
		bodies[i].x += dt * bodies[i].vx
		bodies[i].y += dt * bodies[i].vy
		bodies[i].z += dt * bodies[i].vz
	}
}

func energy(bodies []body) float64 {
	var e float64
	for i, b := range bodies {
		e += 0.5 * b.mass * (b.vx*b.vx + b.vy*b.vy + b.vz*b.vz)
		for _, o := range bodies[i+1:] {
			dx := b.x - o.x
			dy := b.y - o.y
			dz := b.z - o.z
			e -= b.mass * o.mass / math.Sqrt(dx*dx+dy*dy+dz*dz)
		}
	}
	return e
}

func main() {
	bodies := system()
	fmt.Println(int64(math.Floor(energy(bodies) * 1e9)))
	for range 5000000 {
		advance(bodies, 0.01)
	}
	fmt.Println(int64(math.Floor(energy(bodies) * 1e9)))
}
