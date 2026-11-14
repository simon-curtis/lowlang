import * as THREE from "three";
import { RoundedBoxGeometry } from "three/addons/geometries/RoundedBoxGeometry.js";
import { EffectComposer } from "three/addons/postprocessing/EffectComposer.js";
import { RenderPass } from "three/addons/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/addons/postprocessing/UnrealBloomPass.js";
import { OutputPass } from "three/addons/postprocessing/OutputPass.js";
import { ShaderPass } from "three/addons/postprocessing/ShaderPass.js";
import { RoomEnvironment } from "three/addons/environments/RoomEnvironment.js";
import { between, clamp, easeInOut, easeOut, hermite, random, smoothstep, spring } from "./motion.js";
import { highlight, instruction, irRules, lex, mainAssembly, parse } from "./parse.js";
import { compilation, loadFonts, markPolygons, origin, tagline, theme } from "./sources.js";

const width = 1200;
const height = 630;
const fps = 30;
const duration = 12.5;
const scale = Number(new URLSearchParams(location.search).get("scale") ?? 1);

const lineHeight = 0.36;
const lineGap = 0.56;
const pillHeight = 0.62;
const byteWidth = 0.6;
const markHeight = 6;
const markCentre = new THREE.Vector3(14, 0.6, -5);
const finalFov = 35;
const finalDistance = 17.6;
const finalShift = 4.5;
const flight = 1;
const sparkKinds = ["type", "string", "number", "function", "module"];

function textMesh(spans, palette, size) {
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  const font = "500 64px 'JetBrains Mono'";
  context.font = font;
  const joined = spans.map((span) => span.text).join("");
  canvas.width = Math.ceil(context.measureText(joined).width) + 16;
  canvas.height = 96;
  context.font = font;
  context.textBaseline = "middle";
  let x = 8;
  for (const span of spans) {
    context.fillStyle = palette[span.kind];
    context.fillText(span.text, x, 50);
    x += context.measureText(span.text).width;
  }
  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.anisotropy = 8;
  const span = (canvas.width / canvas.height) * size;
  const geometry = new THREE.PlaneGeometry(span, size);
  geometry.translate(span / 2, 0, 0);
  const material = new THREE.MeshBasicMaterial({ map: texture, transparent: true, depthWrite: false, toneMapped: false });
  const mesh = new THREE.Mesh(geometry, material);
  return { mesh, width: span, texture };
}

const dither = {
  uniforms: { tDiffuse: { value: null } },
  vertexShader: "varying vec2 vUv; void main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }",
  fragmentShader: [
    "uniform sampler2D tDiffuse;",
    "varying vec2 vUv;",
    "float hash(vec2 p) { return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453); }",
    "void main() {",
    "  vec4 colour = texture2D(tDiffuse, vUv);",
    "  float noise = hash(gl_FragCoord.xy) + hash(gl_FragCoord.yx + 17.0) - 1.0;",
    "  gl_FragColor = vec4(colour.rgb + noise * 1.5 / 255.0, colour.a);",
    "}",
  ].join("\n"),
};

function reveal(line, amount) {
  const shown = Math.max(amount, 0.0001);
  line.mesh.scale.x = shown;
  line.texture.repeat.x = shown;
}

function backdrop() {
  const canvas = document.createElement("canvas");
  canvas.width = 960;
  canvas.height = 504;
  const context = canvas.getContext("2d");
  const glow = context.createRadialGradient(480, 230, 40, 480, 252, 620);
  glow.addColorStop(0, "#1a1a1a");
  glow.addColorStop(1, "#0c0c0c");
  context.fillStyle = glow;
  context.fillRect(0, 0, canvas.width, canvas.height);
  const texture = new THREE.CanvasTexture(canvas);
  texture.colorSpace = THREE.SRGBColorSpace;
  return texture;
}

function inside(polygon, x, y) {
  let crossing = false;
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const [xi, yi] = polygon[i];
    const [xj, yj] = polygon[j];
    if ((yi > y) !== (yj > y) && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) {
      crossing = !crossing;
    }
  }
  return crossing;
}

function voxels(mark) {
  const grid = 28;
  const cells = [];
  for (let row = 0; row < grid; row++) {
    for (let col = 0; col < grid; col++) {
      const x = ((col + 0.5) * mark.size) / grid;
      const y = ((row + 0.5) * mark.size) / grid;
      if (mark.polygons.some((polygon) => inside(polygon, x, y))) {
        cells.push({ col, row });
      }
    }
  }
  const size = markHeight / grid;
  const gap = new THREE.Vector2(grid * 0.33, grid * 0.66);
  const reach = Math.hypot(grid, grid);
  const targets = [];
  for (const layer of [-0.5, 0.5]) {
    for (const cell of cells) {
      const position = new THREE.Vector3(
        markCentre.x + (cell.col + 0.5 - grid / 2) * size,
        markCentre.y - (cell.row + 0.5 - grid / 2) * size,
        markCentre.z + layer * size,
      );
      const order = Math.hypot(cell.col + 0.5 - gap.x, cell.row + 0.5 - gap.y) / reach;
      targets.push({ position, order });
    }
  }
  return { targets, size, gap: new THREE.Vector3(markCentre.x + (gap.x - grid / 2) * size, markCentre.y - (gap.y - grid / 2) * size, markCentre.z) };
}

function layoutTree(nodes) {
  let slots = 0;
  let deepest = 0;
  const place = (node, depth) => {
    node.depth = depth;
    deepest = Math.max(deepest, depth);
    for (const child of node.children) {
      place(child, depth + 1);
    }
    const childSlots = node.children.map((child) => child.slot);
    node.slot = childSlots.length ? childSlots.reduce((sum, slot) => sum + slot, 0) / childSlots.length : slots++;
  };
  for (const root of nodes.filter((node) => !node.parent)) {
    place(root, 0);
  }
  for (const node of nodes) {
    node.centre = new THREE.Vector3((node.slot - (slots - 1) / 2) * 3.4, (deepest / 2 - node.depth) * 1.45, 0);
  }
}

function besideMark(x, y, z) {
  const offset = new THREE.Vector3(x, y, z);
  return offset.add(markCentre);
}

function cameraKeys() {
  return [
    { t: 0, position: new THREE.Vector3(0.6, 0.9, 10.4), target: new THREE.Vector3(0, 0, 0), fov: 34 },
    { t: 1.7, position: new THREE.Vector3(-0.4, 0.5, 11.2), target: new THREE.Vector3(0, 0.1, 0), fov: 35 },
    { t: 3.3, position: new THREE.Vector3(1.6, 0.4, 11.6), target: new THREE.Vector3(0.2, 0, 0), fov: 36 },
    { t: 4.6, position: new THREE.Vector3(0.3, 0.1, 11), target: new THREE.Vector3(0, 0, 0), fov: 35 },
    { t: 5.95, position: new THREE.Vector3(-0.6, 0.3, 10.6), target: new THREE.Vector3(0.1, 0, 0), fov: 35 },
    { t: 7.2, position: new THREE.Vector3(0.4, 0.5, 10.6), target: new THREE.Vector3(0.3, 0, 0), fov: 35 },
    { t: 8.6, position: new THREE.Vector3(3, 2, 14), target: new THREE.Vector3(5.5, 0.4, -2), fov: 40 },
    { t: 10.1, position: besideMark(finalShift + 4.2, 1.2, finalDistance), target: besideMark(finalShift, 0, 0), fov: finalFov },
    { t: duration, position: besideMark(finalShift + 3.2, 1, finalDistance - 0.6), target: besideMark(finalShift, 0, 0), fov: finalFov },
  ];
}

function wrap(context, words, limit) {
  const lines = [];
  let current = "";
  for (const word of words.split(/\s+/)) {
    const attempt = current ? `${current} ${word}` : word;
    if (current && context.measureText(attempt).width > limit) {
      lines.push(current);
      current = word;
    } else {
      current = attempt;
    }
  }
  lines.push(current);
  return lines;
}

class Film {
  constructor(inputs) {
    this.palette = inputs.palette;
    this.address = inputs.address;
    this.tagline = inputs.tagline;
    this.stages = [
      { name: "lex", detail: "tokens", from: -1, to: 1.75 },
      { name: "parse", detail: "syntax tree", from: 1.75, to: 3.35 },
      { name: "check", detail: "types", from: 3.35, to: 4.6 },
      { name: "ssa", detail: "intermediate form", from: 4.6, to: 5.95 },
      { name: "amd64", detail: "machine code", from: 5.95, to: 7.2 },
      { name: "link", detail: `${inputs.compiled.size.toLocaleString("en")} byte static ELF`, from: 7.2, to: 9.4 },
    ];
    this.renderer = new THREE.WebGLRenderer({ antialias: false, preserveDrawingBuffer: true });
    this.renderer.setPixelRatio(scale);
    this.renderer.setSize(width, height);
    this.output = document.createElement("canvas");
    this.output.width = width * scale;
    this.output.height = height * scale;
    this.context = this.output.getContext("2d");
    this.scene = new THREE.Scene();
    this.scene.background = backdrop();
    this.camera = new THREE.PerspectiveCamera(35, width / height, 0.5, 120);
    this.keys = cameraKeys();
    this.waypoints = [];
    this.light();
    this.tokens(inputs.compiled.source);
    this.tree();
    this.intermediate(inputs.compiled.ir);
    this.assembly(inputs.compiled.assembly);
    this.binary(inputs.compiled.header);
    this.cubes(inputs.mark);
    this.spark();
    this.composer = new EffectComposer(this.renderer);
    this.composer.setPixelRatio(scale);
    this.composer.setSize(width, height);
    const render = new RenderPass(this.scene, this.camera);
    const resolution = new THREE.Vector2(width, height);
    this.bloom = new UnrealBloomPass(resolution, 0.45, 0.4, 1);
    const output = new OutputPass();
    const grain = new ShaderPass(dither);
    for (const pass of [render, this.bloom, output, grain]) {
      this.composer.addPass(pass);
    }
  }

  waypoint(t, position, hop) {
    const point = { t, position, hop };
    this.waypoints.push(point);
  }

  light() {
    const room = new RoomEnvironment();
    this.scene.environment = new THREE.PMREMGenerator(this.renderer).fromScene(room, 0.04).texture;
    this.scene.environmentIntensity = 0.35;
    const sky = new THREE.HemisphereLight(0xffffff, 0x202020, 0.4);
    this.scene.add(sky);
    const key = new THREE.DirectionalLight(0xffffff, 1.15);
    key.position.set(6, 10, 12);
    const rim = new THREE.DirectionalLight(this.palette.type, 1.2);
    rim.position.set(-10, 4, -10);
    this.sweep = new THREE.PointLight(0xffffff, 0, 9, 2);
    this.sweepOffset = new THREE.Vector3();
    this.scene.add(key, rim, this.sweep);
  }

  tokens(source) {
    const tokens = lex(source);
    const gap = 0.12;
    const labels = tokens.map((token) => textMesh([token], this.palette, lineHeight));
    const widths = labels.map((label) => label.width + 0.34);
    const row = widths.reduce((sum, size) => sum + size, 0) + gap * (tokens.length - 1);
    let x = -row / 2;
    this.pills = tokens.map((token, index) => {
      const pillWidth = widths[index];
      const body = new RoundedBoxGeometry(pillWidth, pillHeight, 0.3, 4, 0.16);
      const material = new THREE.MeshPhysicalMaterial({ color: 0x1d1d1d, roughness: 0.4, clearcoat: 0.6, clearcoatRoughness: 0.3, transparent: true });
      const pill = new THREE.Mesh(body, material);
      const label = labels[index];
      label.mesh.position.set(-label.width / 2, 0, 0.152);
      const group = new THREE.Group();
      group.add(pill, label.mesh);
      this.scene.add(group);
      const home = new THREE.Vector3(x + pillWidth / 2, 0, 0);
      x += pillWidth + gap;
      const arrive = -0.3 + index * 0.12;
      const above = home.clone().setY(0.62);
      this.waypoint(arrive + 0.22, above, 0.35);
      const glow = new THREE.Color(this.palette[token.kind]);
      return { token, group, material, label, width: pillWidth, home, arrive, glow };
    });
  }

  tree() {
    this.nodes = parse(this.pills.map((pill) => pill.token));
    layoutTree(this.nodes);
    const gap = 0.06;
    this.nodes.forEach((node, index) => {
      node.start = 1.85 + index * 0.1;
      node.checked = 3.45 + index * 0.2;
      node.collapse = 4.6 + index * 0.05;
      const ordered = [...node.tokens].sort((a, b) => a - b);
      const span = ordered.reduce((sum, token) => sum + this.pills[token].width, 0) + gap * (ordered.length - 1);
      let x = node.centre.x - span / 2;
      for (const token of ordered) {
        const pill = this.pills[token];
        pill.node = node;
        pill.tree = new THREE.Vector3(x + pill.width / 2, node.centre.y, 0);
        x += pill.width + gap;
      }
      node.right = node.centre.x + span / 2;
    });
    const stalk = new THREE.CylinderGeometry(0.013, 0.013, 1, 8);
    stalk.translate(0, 0.5, 0);
    const up = new THREE.Vector3(0, 1, 0);
    this.edges = this.nodes.filter((node) => node.parent).map((node, index) => {
      const from = node.parent.centre.clone().setY(node.parent.centre.y - pillHeight / 2 - 0.04);
      const to = node.centre.clone().setY(node.centre.y + pillHeight / 2 + 0.04);
      const direction = to.clone().sub(from);
      const colour = new THREE.Color(this.palette.paper).multiplyScalar(0.6);
      const material = new THREE.MeshBasicMaterial({ color: colour, transparent: true, depthWrite: false, toneMapped: false });
      const mesh = new THREE.Mesh(stalk, material);
      mesh.position.copy(from);
      const unit = direction.clone().normalize();
      mesh.quaternion.setFromUnitVectors(up, unit);
      this.scene.add(mesh);
      const start = 2.45 + index * 0.16;
      const tip = to.clone().setY(node.centre.y + 0.55);
      this.waypoint(start + 0.38, tip, 0);
      return { mesh, material, length: direction.length(), start };
    });
    const root = this.nodes[0].centre;
    const crown = root.clone().setY(root.y + 0.6);
    this.waypoint(this.edges.at(-1).start + 0.7, crown, 0.2);
    this.chips = this.nodes.map((node) => {
      const spans = [{ text: ": ", kind: "operator" }, { text: node.type, kind: "type" }];
      const chip = textMesh(spans, this.palette, 0.3);
      chip.mesh.position.set(node.right + 0.14, node.centre.y, 0.05);
      this.scene.add(chip.mesh);
      const spot = new THREE.Vector3(node.right + 0.1, node.centre.y + 0.42, 0.1);
      this.waypoint(node.checked - 0.04, spot, 0.3);
      return { ...chip, node };
    });
  }

  panel(lines, top) {
    const left = -Math.max(...lines.map((line) => line.width)) / 2;
    lines.forEach((line, index) => {
      line.home = new THREE.Vector3(left, top - index * lineGap, 0);
      line.mesh.position.copy(line.home);
      this.scene.add(line.mesh);
    });
    return left;
  }

  intermediate(ir) {
    const rows = ir.trimEnd().split("\n");
    this.ir = rows.map((row) => {
      const spans = highlight(row, irRules);
      return textMesh(spans, this.palette, lineHeight);
    });
    const top = ((rows.length - 1) * lineGap) / 2;
    const left = this.panel(this.ir, top);
    this.ir.forEach((line, index) => {
      line.start = 4.85 + index * 0.13;
      line.exit = 5.9 + index * 0.025;
      const margin = new THREE.Vector3(left - 0.3, line.home.y, 0.1);
      this.waypoint(line.start + 0.05, margin, 0);
    });
    this.pills.forEach((pill) => {
      const line = this.ir[Math.min(this.nodes.indexOf(pill.node), this.ir.length - 1)];
      pill.fold = line.home.clone().setX(line.home.x + 0.6);
    });
  }

  assembly(text) {
    const body = mainAssembly(text).map((row) => [{ text: "    ", kind: "operator" }, ...instruction(row)]);
    const rows = [[{ text: "fn_main:", kind: "function" }], ...body];
    this.asm = rows.map((spans) => textMesh(spans, this.palette, lineHeight));
    const top = ((rows.length - 1) * lineGap) / 2;
    const left = this.panel(this.asm, top);
    this.asm.forEach((line, index) => {
      line.start = 6.1 + index * 0.09;
      line.exit = 7 + index * 0.02;
      const margin = new THREE.Vector3(left - 0.3, line.home.y, 0.1);
      this.waypoint(line.start + 0.05, margin, 0);
    });
  }

  binary(header) {
    const columns = 16;
    const rows = Math.ceil(header.length / columns);
    this.bytes = header.map((value, index) => {
      const text = value.toString(16).padStart(2, "0");
      const kind = index < 4 ? "keyword" : "comment";
      const spans = [{ text, kind }];
      const label = textMesh(spans, this.palette, 0.42);
      const col = index % columns;
      const row = Math.floor(index / columns);
      const cell = new THREE.Vector3((col - (columns - 1) / 2) * byteWidth, ((rows - 1) / 2 - row) * lineGap, 0);
      label.mesh.position.set(cell.x - label.width / 2, cell.y, 0);
      this.scene.add(label.mesh);
      return { ...label, cell, pop: 7.4 + index * 0.008, leave: Infinity };
    });
    const reach = (columns / 2) * byteWidth;
    const rise = ((rows - 1) / 2) * lineGap + 0.4;
    const entry = new THREE.Vector3(-reach, rise, 0.1);
    const exit = new THREE.Vector3(reach, -rise, 0.1);
    this.waypoint(7.4, entry, 0);
    this.waypoint(8, exit, 0.6);
  }

  cubes(mark) {
    const { targets, size, gap } = voxels(mark);
    this.cell = size;
    const roll = random(1993);
    const count = targets.length;
    this.particles = targets.map((target, index) => {
      const source = this.bytes[Math.floor(roll() * this.bytes.length)];
      const launch = 8.1 + target.order * 1.05 + roll() * 0.2;
      source.leave = Math.min(source.leave, launch);
      const middle = source.cell.clone().lerp(target.position, 0.5);
      const lift = new THREE.Vector3((roll() - 0.5) * 3, 2.5 + roll() * 2.5, 1.5 + roll() * 2.5);
      const kind = sparkKinds[Math.floor(roll() * sparkKinds.length)];
      return {
        origin: source.cell,
        control: middle.add(lift),
        target: target.position,
        order: target.order,
        launch,
        colour: new THREE.Color(this.palette[kind]),
        tumble: new THREE.Vector3(roll() - 0.5, roll() - 0.5, roll() - 0.5).normalize(),
        spin: (roll() - 0.5) * 6,
        index,
      };
    });
    const box = new THREE.BoxGeometry(1, 1, 1);
    const surface = new THREE.MeshStandardMaterial({ color: 0xffffff, roughness: 0.32, metalness: 0.05 });
    this.blocks = new THREE.InstancedMesh(box, surface, count);
    this.blocks.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    const blank = new THREE.Color();
    this.blocks.setColorAt(0, blank);
    this.blocks.instanceColor.setUsage(THREE.DynamicDrawUsage);
    this.blocks.frustumCulled = false;
    this.scene.add(this.blocks);
    this.scratch = {
      position: new THREE.Vector3(),
      quaternion: new THREE.Quaternion(),
      scale: new THREE.Vector3(),
      matrix: new THREE.Matrix4(),
      colour: new THREE.Color(),
      white: new THREE.Color(this.palette.paper),
    };
    const lift = new THREE.Vector3(0, 3.5, 3);
    const above = gap.clone().add(lift);
    this.waypoint(8.7, above, 0);
    this.waypoint(9.75, gap, 0);
  }

  spark() {
    this.waypoints.sort((a, b) => a.t - b.t);
    const colour = new THREE.Color(this.palette.paper).multiplyScalar(2.2);
    this.trail = Array.from({ length: 7 }, (_, index) => {
      const geometry = new THREE.SphereGeometry(0.065 * (1 - index * 0.11), 16, 12);
      const material = new THREE.MeshBasicMaterial({ color: colour, transparent: true, toneMapped: false, depthWrite: false });
      const mesh = new THREE.Mesh(geometry, material);
      this.scene.add(mesh);
      return { mesh, material, lag: index * 0.016, strength: 1 - index * 0.14 };
    });
  }

  sparkAt(t) {
    const points = this.waypoints;
    if (t <= points[0].t) {
      return points[0].position.clone();
    }
    const next = points.findIndex((point) => point.t > t);
    if (next < 0) {
      return points.at(-1).position.clone();
    }
    const from = points[next - 1];
    const to = points[next];
    const u = easeInOut((t - from.t) / (to.t - from.t));
    const position = from.position.clone().lerp(to.position, u);
    position.y += Math.sin(Math.PI * u) * to.hop;
    return position;
  }

  animateTokens(t) {
    for (const [index, pill] of this.pills.entries()) {
      const node = pill.node;
      const arrived = spring((t - pill.arrive) / 0.6);
      const built = spring((t - node.start) / 0.75);
      const folded = easeInOut((t - node.collapse) / 0.45);
      const position = pill.home.clone().lerp(pill.tree, built).lerp(pill.fold, folded);
      position.y += (1 - arrived) * 1.4 + Math.sin(t * 1.4 + index) * 0.025;
      pill.group.position.copy(position);
      pill.group.rotation.set((1 - arrived) * -0.7, Math.sin(Math.PI * clamp(built)) * 0.35, 0);
      pill.group.scale.setScalar(1 - 0.55 * folded);
      const visible = clamp((t - pill.arrive) / 0.2) * (1 - folded);
      pill.group.visible = visible > 0.002;
      pill.material.depthWrite = visible > 0.98;
      pill.material.opacity = visible;
      pill.label.mesh.material.opacity = visible;
      const pulse = Math.exp(-(((t - node.checked) / 0.14) ** 2));
      pill.material.emissive.copy(pill.glow).multiplyScalar(pulse * 0.35);
    }
  }

  animateTree(t) {
    const fade = 1 - smoothstep(4.6, 4.9, t);
    for (const edge of this.edges) {
      const grown = easeOut((t - edge.start) / 0.38);
      edge.mesh.scale.set(1, Math.max(grown, 0.0001) * edge.length, 1);
      edge.material.opacity = clamp(grown * 4) * fade;
    }
    for (const chip of this.chips) {
      const popped = spring((t - chip.node.checked) / 0.5);
      chip.mesh.scale.setScalar(Math.max(popped, 0.0001));
      chip.mesh.material.opacity = clamp(popped) * fade;
    }
  }

  animatePanel(lines, t, rise) {
    for (const line of lines) {
      const shown = easeOut((t - line.start) / 0.42);
      const gone = easeInOut((t - line.exit) / 0.35);
      reveal(line, shown);
      line.mesh.position.copy(line.home);
      line.mesh.position.y += (1 - easeOut((t - line.start) / 0.5)) * -rise + gone * 2.4;
      line.mesh.material.opacity = clamp((t - line.start) / 0.15) * (1 - gone);
    }
  }

  animateBytes(t) {
    for (const byte of this.bytes) {
      const popped = spring((t - byte.pop) / 0.45);
      byte.mesh.scale.setScalar(Math.max(popped, 0.0001));
      byte.mesh.position.x = byte.cell.x - (byte.width * Math.max(popped, 0.0001)) / 2;
      byte.mesh.material.opacity = clamp(popped) * (1 - smoothstep(byte.leave, byte.leave + 0.5, t));
    }
  }

  animateCubes(t) {
    const scratch = this.scratch;
    for (const particle of this.particles) {
      const progress = clamp((t - particle.launch) / flight);
      const eased = easeInOut(progress);
      const inverse = 1 - eased;
      scratch.position.copy(particle.origin).multiplyScalar(inverse * inverse);
      const bend = particle.control.clone().multiplyScalar(2 * inverse * eased);
      const end = particle.target.clone().multiplyScalar(eased * eased);
      scratch.position.add(bend).add(end);
      const landed = t - particle.launch - flight;
      const flash = landed > 0 ? Math.exp(-landed * 6) * 1.3 : 0;
      const ripple = Math.exp(-(((t - 9.95 - particle.order * 0.7) / 0.09) ** 2)) * 0.9;
      const appear = smoothstep(particle.launch, particle.launch + 0.18, t);
      const fuse = 0.84 + 0.16 * smoothstep(10.2, 11, t);
      scratch.quaternion.setFromAxisAngle(particle.tumble, inverse * inverse * particle.spin * 2);
      scratch.scale.setScalar(this.cell * fuse * appear);
      scratch.matrix.compose(scratch.position, scratch.quaternion, scratch.scale);
      scratch.colour.copy(particle.colour).lerp(scratch.white, smoothstep(0.55, 1, progress)).multiplyScalar(0.95 + flash + ripple);
      this.blocks.setMatrixAt(particle.index, scratch.matrix);
      this.blocks.setColorAt(particle.index, scratch.colour);
    }
    this.blocks.instanceMatrix.needsUpdate = true;
    this.blocks.instanceColor.needsUpdate = true;
  }

  animateSpark(t) {
    const shown = between(t, -1, 9.95, 0.25);
    for (const ghost of this.trail) {
      const position = this.sparkAt(t - ghost.lag);
      ghost.mesh.position.copy(position);
      ghost.material.opacity = shown * ghost.strength;
    }
  }

  stage(t) {
    this.animateTokens(t);
    this.animateTree(t);
    this.animatePanel(this.ir, t, 0);
    this.animatePanel(this.asm, t, 0.8);
    this.animateBytes(t);
    this.animateCubes(t);
    this.animateSpark(t);
    this.bloom.strength = 0.45 + 0.2 * Math.exp(-(((t - 10) / 0.35) ** 2));
    const pass = smoothstep(11, 12.3, t);
    this.sweepOffset.set(-5 + pass * 10, 1.5 - pass * 3, 2.4);
    this.sweep.position.copy(markCentre).add(this.sweepOffset);
    this.sweep.intensity = Math.sin(Math.PI * pass) * 6;
  }

  aim(t) {
    const position = hermite(this.keys, t, (key) => key.position);
    const target = hermite(this.keys, t, (key) => key.target);
    const fov = hermite(this.keys, t, (key) => new THREE.Vector3(key.fov, 0, 0)).x;
    this.camera.position.copy(position);
    this.camera.fov = fov;
    this.camera.updateProjectionMatrix();
    this.camera.lookAt(target);
  }

  captions(t) {
    const context = this.context;
    const left = 56;
    const shown = 1 - smoothstep(9.2, 9.6, t);
    context.font = "500 17px 'JetBrains Mono'";
    context.letterSpacing = "0px";
    this.stages.forEach((stage, index) => {
      const active = between(t, stage.from, stage.to, 0.22);
      context.globalAlpha = shown * (0.22 + 0.78 * active);
      context.fillStyle = this.palette.paper;
      context.fillRect(left + index * 34, 548, 26, 2);
      context.globalAlpha = shown * active;
      context.fillStyle = this.palette.comment;
      const number = `0${index + 1}`;
      context.fillText(number, left, 580);
      const offset = context.measureText(`${number}  `).width;
      context.fillStyle = this.palette.paper;
      context.fillText(stage.name, left + offset, 580);
      const after = context.measureText(`${stage.name}  `).width;
      context.fillStyle = this.palette.comment;
      context.fillText(stage.detail, left + offset + after, 580);
    });
  }

  title(t) {
    const context = this.context;
    const left = 610;
    context.font = "700 132px Inter";
    context.letterSpacing = "-3px";
    context.textBaseline = "alphabetic";
    context.fillStyle = this.palette.paper;
    let x = left;
    [..."Low"].forEach((letter, index) => {
      const shown = easeOut((t - 10.1 - index * 0.09) / 0.6);
      context.globalAlpha = shown;
      context.filter = `blur(${((1 - shown) * 10).toFixed(2)}px)`;
      context.fillText(letter, x, 292 + (1 - shown) * 48);
      x += context.measureText(letter).width - 3;
    });
    context.letterSpacing = "0px";
    context.font = "400 29px Inter";
    wrap(context, this.tagline, 500).forEach((words, index) => {
      const shown = easeOut((t - 10.55 - index * 0.12) / 0.6);
      context.globalAlpha = shown * 0.8;
      context.filter = `blur(${((1 - shown) * 6).toFixed(2)}px)`;
      context.fillText(words, left + 4, 352 + index * 42 + (1 - shown) * 24);
    });
    const shown = easeOut((t - 11.05) / 0.6);
    context.globalAlpha = shown;
    context.filter = "none";
    context.font = "500 20px 'JetBrains Mono'";
    context.fillStyle = this.palette.string;
    context.fillText(this.address, left + 4, 520 + (1 - shown) * 16);
  }

  frame(t) {
    this.stage(t);
    this.aim(t);
    this.composer.render();
    const context = this.context;
    context.globalAlpha = 1;
    context.filter = "none";
    context.drawImage(this.renderer.domElement, 0, 0);
    context.save();
    context.scale(scale, scale);
    context.textBaseline = "alphabetic";
    this.captions(t);
    this.title(t);
    context.restore();
    return this.output.toDataURL("image/png");
  }
}

async function prepare() {
  await loadFonts();
  const [compiled, mark, palette, lead, address] = await Promise.all([compilation(), markPolygons(), theme(), tagline(), origin()]);
  return new Film({ compiled, mark, palette, tagline: lead, address });
}

const film = prepare();

window.scene = {
  width,
  height,
  fps,
  duration,
  ready: film,
  async frame(t) {
    const ready = await film;
    return ready.frame(t);
  },
};
