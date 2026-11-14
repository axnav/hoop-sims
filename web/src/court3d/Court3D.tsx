// 3D court view. Loaded lazily (see GameViewer). Game state comes from replay.ts (the same Replay as the 2D view);
// the scene reads the current event index from a ref inside useFrame, so playback causes no React re-renders.
import { useEffect, useMemo, useRef, type MutableRefObject } from 'react'
import { Canvas, useFrame, useThree } from '@react-three/fiber'
import { Html, OrbitControls } from '@react-three/drei'
import * as THREE from 'three'
import type { Replay } from '../replay'

export type CamName = 'broadcast' | 'baseline' | 'overhead'
export type CamCmd = { name: CamName; n: number }
export type TeamInfo = { id: number; primary: string; secondary: string }
type Props = {
  replay: Replay; home: TeamInfo; away: TeamInfo; names: Map<number, string>
  idxRef: MutableRefObject<number>; speedRef: MutableRefObject<number>; camRef: MutableRefObject<CamCmd>
  reduceMotion: boolean
}

const L = 28.65, W = 15.24, HX = 12.75, RIM_Y = 3.05
const PW = L + 3, PD = W + 3
const CAMS: Record<CamName, { pos: [number, number, number]; target: [number, number, number] }> = {
  broadcast: { pos: [0, 12, 21], target: [0, 0.5, 0] },
  baseline: { pos: [-23, 4.5, 0.01], target: [3, 1.2, 0] },
  overhead: { pos: [0, 34, 0.01], target: [0, 0, 0] },
}

function makeFloor(primary: string): THREE.CanvasTexture {
  const S = 64
  const c = document.createElement('canvas')
  c.width = Math.round(PW * S); c.height = Math.round(PD * S)
  const g = c.getContext('2d')!
  const X = (x: number) => (x + PW / 2) * S, Z = (z: number) => (z + PD / 2) * S
  g.fillStyle = '#a77f4a'; g.fillRect(0, 0, c.width, c.height)
  g.fillStyle = '#c9a063'; g.fillRect(X(-L / 2), Z(-W / 2), L * S, W * S)
  for (let z = -PD / 2, k = 0; z < PD / 2; z += 0.14, k++) {
    g.fillStyle = `rgba(${k % 3 === 0 ? '60,35,10' : '255,230,170'},${k % 3 === 0 ? 0.16 : 0.05})`
    g.fillRect(0, Z(z), c.width, k % 3 === 0 ? 2 : 4)
  }
  g.fillStyle = primary; g.globalAlpha = 0.92
  for (const s of [-1, 1]) g.fillRect(X(s < 0 ? -L / 2 : L / 2 - 5.79), Z(-2.44), 5.79 * S, 4.88 * S)
  g.beginPath(); g.arc(X(0), Z(0), 1.83 * S, 0, 7); g.fill()
  g.globalAlpha = 1
  g.strokeStyle = '#fff'; g.lineWidth = 0.06 * S; g.lineJoin = 'round'
  g.strokeRect(X(-L / 2), Z(-W / 2), L * S, W * S)
  g.beginPath(); g.moveTo(X(0), Z(-W / 2)); g.lineTo(X(0), Z(W / 2)); g.stroke()
  g.beginPath(); g.arc(X(0), Z(0), 1.83 * S, 0, 7); g.stroke()
  const a0 = Math.asin(6.7 / 7.24), dx0 = 7.24 * Math.cos(a0)
  for (const s of [-1, 1]) {
    const base = s * L / 2, lineX = s * (L / 2 - 5.79), cx = s * HX
    g.strokeRect(Math.min(X(base), X(lineX)), Z(-2.44), 5.79 * S, 4.88 * S)
    g.beginPath(); g.arc(X(lineX), Z(0), 1.83 * S, 0, 7); g.stroke()
    g.beginPath(); g.arc(X(cx), Z(0), 0.6 * S, 0, 7); g.stroke()
    g.beginPath(); g.moveTo(X(base), Z(-6.7)); g.lineTo(X(cx - s * dx0), Z(-6.7))
    for (let a = -a0; a <= a0 + 1e-6; a += 0.04) g.lineTo(X(cx - s * 7.24 * Math.cos(a)), Z(7.24 * Math.sin(a)))
    g.lineTo(X(base), Z(6.7)); g.stroke()
  }
  const t = new THREE.CanvasTexture(c)
  t.colorSpace = THREE.SRGBColorSpace; t.anisotropy = 8
  return t
}

function Hoop({ sgn, netRef }: { sgn: number; netRef: (g: THREE.Group | null) => void }) {
  return (
    <group position={[sgn * HX, 0, 0]}>
      <mesh position={[sgn * 0.375, 3.435, 0]}><boxGeometry args={[0.05, 1.07, 1.83]} /><meshStandardMaterial color="#e9eef5" transparent opacity={0.85} /></mesh>
      <mesh position={[sgn * 0.375, 3.2, 0]}><boxGeometry args={[0.06, 0.45, 0.59]} /><meshBasicMaterial color="#dd3b2a" wireframe /></mesh>
      <mesh position={[0, RIM_Y, 0]} rotation-x={Math.PI / 2}><torusGeometry args={[0.225, 0.014, 8, 28]} /><meshStandardMaterial color="#ff6a1a" /></mesh>
      <group ref={netRef} position={[0, RIM_Y, 0]}>
        <mesh position={[0, -0.21, 0]}><cylinderGeometry args={[0.225, 0.13, 0.42, 12, 3, true]} /><meshBasicMaterial color="#ffffff" wireframe transparent opacity={0.8} /></mesh>
      </group>
      <mesh position={[sgn * (L / 2 + 1 - HX), 1.7, 0]}><cylinderGeometry args={[0.1, 0.12, 3.4, 8]} /><meshStandardMaterial color="#2a3040" /></mesh>
      <mesh position={[sgn * ((L / 2 + 1 - HX + 0.375) / 2), 3.3, 0]}><boxGeometry args={[L / 2 + 1 - HX - 0.375, 0.1, 0.1]} /><meshStandardMaterial color="#2a3040" /></mesh>
    </group>
  )
}

function hash(n: number, k = 0) {
  let x = (Math.imul(n, 374761393) + Math.imul(k, 668265263)) | 0
  x = Math.imul(x ^ (x >>> 13), 1274126177)
  x ^= x >>> 16
  return (x >>> 0) / 4294967296
}
const OFF: [number, number][] = [[8.2, 0.4], [5.4, 5.6], [5.4, -5.6], [1.2, 6.9], [1.9, -2.0]]
const ease = (t: number) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2)

type Seg = { dur: number; from: THREE.Vector3; to: THREE.Vector3; arc: number; pre?: () => void; post?: () => void; started?: boolean }
type Pt = { x: number; z: number }

function Game({ replay, home, away, names, idxRef, speedRef, camRef, reduceMotion }: Props) {
  const teams = [home, away]
  const bodies = useRef<(THREE.MeshStandardMaterial | null)[][]>([[], []])
  const groups = useRef<(THREE.Group | null)[][]>([[], []])
  const nets = useRef<(THREE.Group | null)[]>([null, null])
  const ballRef = useRef<THREE.Mesh>(null)
  const ringRef = useRef<THREE.Mesh>(null)
  const labelG = useRef<THREE.Group>(null)
  const labelEl = useRef<HTMLDivElement>(null)
  const ptsG = useRef<THREE.Group>(null)
  const ptsEl = useRef<HTMLDivElement>(null)
  const ctl = useRef<any>(null)
  const { camera } = useThree()
  const floor = useMemo(() => makeFloor(home.primary), [home.primary])
  useEffect(() => () => floor.dispose(), [floor])

  const S = useRef<any>(null)
  if (!S.current) {
    const slots = [0, 1].map((s) => Array.from({ length: 5 }, (_, k) => replay.sides[s].lines[k]?.id ?? -1))
    const mk = () => Array.from({ length: 5 }, () => ({ x: 0, z: 0 }))
    S.current = {
      last: 0, slots, used: [[0, 1, 2, 3, 4], [0, 1, 2, 3, 4]], tgt: [mk(), mk()], pos: [mk(), mk()], snap: true,
      offSide: 0, hoop: 1, possKey: 0, active: null as null | { side: number; slot: number }, flash: 0,
      plan: null as null | { segs: Seg[]; i: number; t: number }, holder: null as null | { side: number; slot: number },
      rest: new THREE.Vector3(0, 0.15, 0), flick: [0, 0], ptsT: 1, clock: 0, cam: null as null | { t: number; p: THREE.Vector3; tg: THREE.Vector3; p1: THREE.Vector3; t1: THREE.Vector3 },
      camN: 0, tick: 5,
    }
    setFormation(S.current, 0, 1, 0)
  }

  function setFormation(s: any, off: number, hoop: number, key: number) {
    s.offSide = off; s.hoop = hoop; s.possKey = key
    for (let k = 0; k < 5; k++) {
      let dx = OFF[k][0] + (hash(key, k) - 0.5) * 1.4, z = OFF[k][1] + (hash(key, k + 9) - 0.5) * 1.4
      dx = Math.max(0.9, dx); z = Math.max(-7.2, Math.min(7.2, z))
      s.tgt[off][k] = { x: hoop * (HX - dx), z }
      s.tgt[1 - off][k] = { x: hoop * (HX - Math.max(0.7, dx - 1.2)), z: z * 0.85 }
    }
  }
  function getSlot(s: any, side: number, id: number) {
    let k = s.slots[side].indexOf(id)
    if (k < 0) {
      k = s.used[side].indexOf(Math.min(...s.used[side]))
      s.slots[side][k] = id
    }
    s.used[side][k] = ++s.tick
    return k
  }
  const hand = (s: any, side: number, k: number) => new THREE.Vector3(s.tgt[side][k].x, 1.45, s.tgt[side][k].z)
  const ballPos = (s: any) => {
    if (s.holder) { const p = s.pos[s.holder.side][s.holder.k ?? s.holder.slot]; return new THREE.Vector3(p.x + 0.3, 1.1, p.z) }
    return ballRef.current ? ballRef.current.position.clone() : new THREE.Vector3()
  }
  function finishPlan(s: any) {
    const p = s.plan
    if (!p) return
    for (; p.i < p.segs.length; p.i++) {
      const g = p.segs[p.i]
      if (!g.started) g.pre?.()
      ballRef.current?.position.copy(g.to)
      g.post?.()
    }
    s.plan = null
  }

  function startEvent(s: any, i: number, jump: boolean) {
    const e = replay.events[i]
    finishPlan(s)
    const side = replay.sides[0].teamId === e.t ? 0 : 1, other = 1 - side
    const sp = speedRef.current
    const m = sp >= 16 ? 0 : sp >= 4 ? 0.25 : 1
    const homeDir = e.p <= 2 ? 1 : -1
    const dirOf = (q: number) => (q === 0 ? homeDir : -homeDir)
    let off = side, hoop = dirOf(side)
    if (e.y === 'dreb') { off = other; hoop = dirOf(other) }
    if (off !== s.offSide || hoop !== s.hoop) s.possKey = i
    setFormation(s, off, hoop, s.possKey)
    const slot = getSlot(s, side, e.pl)
    s.active = { side, slot }
    s.snap = jump || m === 0
    const shotAt = (d: number, a: number) => ({ x: hoop * (HX - d * Math.cos(a)), z: d * Math.sin(a) })
    const rimTop = new THREE.Vector3(hoop * HX, RIM_Y + 0.05, 0)
    const segs: Seg[] = []
    const add = (dur: number, from: THREE.Vector3, to: THREE.Vector3, arc: number, pre?: () => void, post?: () => void) => {
      segs.push({ dur: dur * m, from, to, arc, pre, post })
    }
    const hid = hoop > 0 ? 1 : 0
    const label = (txt: string, side2: number) => () => {
      if (m === 0 || !ptsEl.current || !ptsG.current) return
      ptsEl.current.textContent = txt
      ptsEl.current.style.color = teams[side2].primary
      ptsG.current.position.set(hoop * HX, RIM_Y + 0.3, 0)
      s.ptsT = 0
    }
    const makeTail = (cur: THREE.Vector3, txt: string) => {
      add(0.05, rimTop, new THREE.Vector3(hoop * HX, 2.55, 0), 0, () => { if (m > 0) s.flick[hid] = 1; label(txt, side)() })
      const fl = new THREE.Vector3(hoop * HX - hoop * 0.2, 0.15, 0)
      add(0.06, new THREE.Vector3(hoop * HX, 2.55, 0), fl, 0, undefined, () => s.rest.copy(fl))
      return cur
    }
    const missTail = () => {
      const to = new THREE.Vector3(hoop * (HX - 1.1), 0.25, (hash(i, 3) - 0.5) * 1.6)
      add(0.14, rimTop, to, 1.1, undefined, () => s.rest.copy(to))
    }
    const toHand = (v: THREE.Vector3, d = 0.06) => add(d, ballPos(s), v, 0.15, () => { s.holder = null })
    if (e.y === '2' || e.y === '3' || e.y === 'm2' || e.y === 'm3') {
      const three = e.y === '3' || e.y === 'm3', made = e.y === '2' || e.y === '3'
      let spot: Pt
      if (three) spot = shotAt(7.6, (hash(i, 1) - 0.5) * 2 * 1.25)
      else if (hash(i, 2) < 0.55) spot = shotAt(0.6 + hash(i, 4) * 0.9, (hash(i, 1) - 0.5) * 2 * 1.0)
      else spot = shotAt(4 + hash(i, 4) * 1.5, (hash(i, 1) - 0.5) * 2 * 1.2)
      s.tgt[side][slot] = spot
      const h = hand(s, side, slot)
      if (made && e.a >= 0) {
        const as = getSlot(s, side, e.a)
        add(0.12, ballPos(s), hand(s, side, as), 0.1, () => { s.holder = null })
        add(0.12, hand(s, side, as), h, 0.4)
      } else toHand(h)
      add(0.26, h, rimTop, 0.9 + Math.hypot(h.x - rimTop.x, h.z) * 0.1)
      if (made) makeTail(h, three ? '+3' : '+2'); else missTail()
    } else if (e.y === 'ft') {
      const spot = { x: hoop * (HX - 4.215), z: 0 }
      s.tgt[side][slot] = spot
      const h = hand(s, side, slot), n = e.n ?? 0
      toHand(h)
      for (let k = 0; k < 2; k++) {
        add(0.14, h, rimTop, 1.1)
        if (k < n) {
          add(0.04, rimTop, new THREE.Vector3(hoop * HX, 2.55, 0), 0, () => { if (m > 0) s.flick[hid] = 1; if (k === n - 1) label(`+${n}`, side)() })
          const fl = new THREE.Vector3(hoop * HX - hoop * 0.2, 0.15, 0)
          add(0.04, new THREE.Vector3(hoop * HX, 2.55, 0), fl, 0, undefined, () => { s.rest.copy(fl); if (k === 0) s.holder = { side, slot } })
        } else missTail()
      }
    } else if (e.y === 'tov') {
      s.flash = 1
      toHand(hand(s, side, slot), 0.06)
      const o = getSlot(s, other, replay.sides[other].lines[0]?.id ?? -1)
      add(0.2, hand(s, side, slot), hand(s, other, o), 0.2, undefined, () => { s.holder = { side: other, slot: o } })
    } else if (e.y === 'oreb' || e.y === 'dreb') {
      s.tgt[side][slot] = { x: s.rest.x - hoop * 0.5, z: s.rest.z }
      const h = hand(s, side, slot)
      add(0.14, ballPos(s), h, 0.3, () => { s.holder = null }, () => { s.holder = { side, slot } })
    }
    s.plan = { segs, i: 0, t: 0 }
    if (labelEl.current) labelEl.current.textContent = names.get(e.pl) ?? ''
  }

  useFrame((_, dtRaw) => {
    const s = S.current, dt = Math.min(dtRaw, 0.1)
    s.clock += dt
    // new events
    const idx = idxRef.current
    if (idx !== s.last) {
      if (idx > 0) startEvent(s, idx - 1, idx - s.last > 2)
      else { finishPlan(s); s.active = null; s.holder = null; s.snap = true; setFormation(s, 0, 1, 0) }
      s.last = idx
    }
    // ball plan
    const ball = ballRef.current
    if (ball) {
      let rem = dt
      const p = s.plan as null | { segs: Seg[]; i: number; t: number }
      while (p && p.i < p.segs.length) {
        const g = p.segs[p.i]
        if (!g.started) { g.started = true; g.pre?.() }
        if (g.dur <= 0 || p.t + rem >= g.dur) {
          rem -= Math.max(0, g.dur - p.t)
          ball.position.copy(g.to); g.post?.(); p.i++; p.t = 0
          continue
        }
        p.t += rem
        const u = p.t / g.dur
        ball.position.lerpVectors(g.from, g.to, u)
        ball.position.y += g.arc * 4 * u * (1 - u)
        break
      }
      if (p && p.i >= p.segs.length) s.plan = null
      if (!s.plan) {
        if (s.holder) {
          const q = s.pos[s.holder.side][s.holder.slot]
          ball.position.set(q.x + 0.3, 1.0 + Math.abs(Math.sin(s.clock * 7)) * 0.45, q.z)
        } else if (s.last === 0) ball.position.set(0, 1.0, 0)
      }
    }
    // figures
    const k = s.snap ? 1 : 1 - Math.exp(-dt * 3.2), kA = s.snap ? 1 : 1 - Math.exp(-dt * 9)
    for (let sd = 0; sd < 2; sd++) for (let j = 0; j < 5; j++) {
      const p = s.pos[sd][j], t = s.tgt[sd][j], kk = s.active && s.active.side === sd && s.active.slot === j ? kA : k
      p.x += (t.x - p.x) * kk; p.z += (t.z - p.z) * kk
      groups.current[sd][j]?.position.set(p.x, 0, p.z)
      const g = groups.current[sd][j]
      if (g) g.rotation.y = Math.atan2(s.hoop * HX - p.x, -p.z)
      const mat = bodies.current[sd][j]
      if (mat) {
        const on = s.active && s.active.side === sd && s.active.slot === j
        mat.emissiveIntensity += ((on ? 0.8 : 0) - mat.emissiveIntensity) * Math.min(1, dt * 10)
      }
    }
    if (s.snap && s.plan === null) s.snap = false
    // highlight ring + name label
    const a = s.active, ring = ringRef.current
    if (ring) {
      ring.visible = !!a
      if (a) {
        const p = s.pos[a.side][a.slot]
        ring.position.set(p.x, 0.04, p.z)
        const pulse = 1 + 0.12 * Math.sin(s.clock * 6) + s.flash * 0.6
        ring.scale.set(pulse, pulse, 1)
        ;(ring.material as THREE.MeshBasicMaterial).color.set(s.flash > 0.05 ? '#ff4455' : '#ffffff')
        labelG.current?.position.set(p.x, 2.55, p.z)
      }
      if (labelG.current) labelG.current.visible = !!a
    }
    s.flash = Math.max(0, s.flash - dt * 1.6)
    // net flick, points label
    for (let h = 0; h < 2; h++) {
      const n = nets.current[h]
      if (!n) continue
      s.flick[h] *= Math.exp(-dt * 5)
      const f = s.flick[h]
      n.scale.set(1 - 0.15 * f, 1 + 0.4 * f * Math.abs(Math.sin(s.clock * 22)), 1 - 0.15 * f)
    }
    if (s.ptsT < 1 && ptsG.current && ptsEl.current) {
      s.ptsT = Math.min(1, s.ptsT + dt / 1.1)
      ptsG.current.position.y = RIM_Y + 0.3 + s.ptsT * 1.2
      ptsEl.current.style.opacity = String(1 - Math.pow(s.ptsT, 3))
      ptsEl.current.style.transform = `scale(${0.8 + 0.5 * Math.min(1, s.ptsT * 4)})`
    } else if (ptsEl.current) ptsEl.current.style.opacity = '0'
    // camera presets
    const cc = camRef.current
    if (cc.n !== s.camN && ctl.current) {
      s.camN = cc.n
      const pr = CAMS[cc.name]
      s.cam = { t: reduceMotion ? 1 : 0, p: camera.position.clone(), tg: ctl.current.target.clone(), p1: new THREE.Vector3(...pr.pos), t1: new THREE.Vector3(...pr.target) }
    }
    if (s.cam && ctl.current) {
      s.cam.t = Math.min(1, s.cam.t + dt / 1.1)
      const e = ease(s.cam.t)
      camera.position.lerpVectors(s.cam.p, s.cam.p1, e)
      ctl.current.target.lerpVectors(s.cam.tg, s.cam.t1, e)
      ctl.current.update()
      if (s.cam.t >= 1) s.cam = null
    }
  })

  const fig = (side: number, k: number) => {
    const t = teams[side]
    return (
      <group key={`${side}-${k}`} ref={(g) => { groups.current[side][k] = g }}>
        <mesh position={[0, 0.95, 0]}>
          <capsuleGeometry args={[0.25, 0.8, 4, 12]} />
          <meshStandardMaterial ref={(m) => { bodies.current[side][k] = m }} color={t.primary} emissive={t.primary} emissiveIntensity={0} roughness={0.7} />
        </mesh>
        <mesh position={[0, 1.15, 0]}><cylinderGeometry args={[0.262, 0.262, 0.14, 14]} /><meshStandardMaterial color={t.secondary} roughness={0.7} /></mesh>
        <mesh position={[0, 1.78, 0]}><sphereGeometry args={[0.17, 14, 12]} /><meshStandardMaterial color="#d9b48f" roughness={0.8} /></mesh>
      </group>
    )
  }

  return (
    <>
      <color attach="background" args={['#06080d']} />
      <fog attach="fog" args={['#06080d', 45, 90]} />
      <hemisphereLight args={['#ffffff', '#202838', 0.9]} />
      <directionalLight position={[8, 22, 10]} intensity={1.6} />
      <pointLight position={[-HX, 9, 0]} intensity={90} distance={30} />
      <pointLight position={[HX, 9, 0]} intensity={90} distance={30} />
      <mesh rotation-x={-Math.PI / 2} position={[0, -0.02, 0]}><circleGeometry args={[70, 48]} /><meshStandardMaterial color="#0b0e15" /></mesh>
      <mesh rotation-x={-Math.PI / 2}><planeGeometry args={[PW, PD]} /><meshStandardMaterial map={floor} roughness={0.55} /></mesh>
      <Hoop sgn={-1} netRef={(g) => { nets.current[0] = g }} />
      <Hoop sgn={1} netRef={(g) => { nets.current[1] = g }} />
      {[0, 1].flatMap((sd) => [0, 1, 2, 3, 4].map((k) => fig(sd, k)))}
      <mesh ref={ballRef} position={[0, 1, 0]}><sphereGeometry args={[0.15, 16, 12]} /><meshStandardMaterial color="#e8761f" roughness={0.6} /></mesh>
      <mesh ref={ringRef} rotation-x={-Math.PI / 2} visible={false}><ringGeometry args={[0.5, 0.62, 32]} /><meshBasicMaterial color="#ffffff" transparent opacity={0.9} /></mesh>
      <group ref={labelG}>
        <Html center zIndexRange={[5, 0]} style={{ pointerEvents: 'none' }}>
          <div ref={labelEl} style={{ background: 'rgba(8,10,16,.85)', color: '#fff', padding: '2px 8px', borderRadius: 6, font: '600 12px Inter, sans-serif', whiteSpace: 'nowrap' }} />
        </Html>
      </group>
      <group ref={ptsG} position={[0, RIM_Y + 0.3, 0]}>
        <Html center zIndexRange={[5, 0]} style={{ pointerEvents: 'none' }}>
          <div ref={ptsEl} style={{ opacity: 0, font: '900 30px Inter, sans-serif', textShadow: '0 0 4px #fff, 0 2px 6px #000' }} />
        </Html>
      </group>
      <OrbitControls ref={ctl} makeDefault target={CAMS.broadcast.target} minDistance={5} maxDistance={48} maxPolarAngle={Math.PI / 2 - 0.04} enablePan={false} enableDamping />
    </>
  )
}

export default function Court3D(props: Props) {
  return (
    <Canvas camera={{ position: CAMS.broadcast.pos, fov: 42, near: 0.1, far: 200 }} dpr={[1, 1.75]} gl={{ antialias: true }}>
      <Game {...props} />
    </Canvas>
  )
}
