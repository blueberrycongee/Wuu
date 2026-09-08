import test from 'node:test';
import assert from 'node:assert/strict';
import { eyePose, projectEye, spinAngle, springStep, blinkLid, hopPose, wigglePose, splitRest, splitEnter, splitExit } from '../mascot-motion.mjs';
const eyes = [{ x: 46.5, y: 56.7 }, { x: 64.5, y: 56 }];

test('spherical projection preserves the authored eyes at rest and after a turn', () => {
  for (const eye of eyes) for (const angle of [0, Math.PI * 2]) {
    const pose = projectEye(eye, angle);
    assert.ok(Math.abs(pose.x - eye.x) < 1e-10);
    assert.ok(Math.abs(pose.y - eye.y) < 1e-10);
    assert.ok(Math.abs(pose.width - 1) < 1e-10);
    assert.equal(pose.opacity, 1);
  }
});

test('eyes cross the silhouette independently rather than disappearing as one sticker', () => {
  const left = projectEye(eyes[0], Math.PI / 2);
  const right = projectEye(eyes[1], Math.PI / 2);
  assert.ok(left.opacity > 0);
  assert.equal(right.opacity, 0);
  assert.ok(left.width > right.width);
  for (const eye of eyes) assert.equal(projectEye(eye, Math.PI).opacity, 0);
});

test('projection remains continuous across both silhouette edges', () => {
  for (const eye of eyes) {
    let previous = projectEye(eye, 0);
    for (let angle = 0.001; angle < Math.PI * 2; angle += 0.001) {
      const current = projectEye(eye, angle);
      assert.ok(Math.abs(current.opacity - previous.opacity) < 0.013);
      assert.ok(Math.abs(current.x - previous.x) < 0.04);
      previous = current;
    }
  }
});

test('turn anticipates and overshoots but finishes at exactly one revolution', () => {
  assert.ok(Math.abs(spinAngle(0)) < 1e-10);
  assert.ok(spinAngle(0.1) < 0);
  assert.ok(spinAngle(0.8) > Math.PI * 2);
  assert.equal(spinAngle(1), Math.PI * 2);
  for (const join of [0.14, 0.78]) assert.ok(Math.abs(spinAngle(join - 1e-6) - spinAngle(join + 1e-6)) < 1e-6);
});

test('glances release into the existing gaze without moving its anchor', () => {
  const gaze = { x: 7, y: -3 };
  for (const action of ['peek', 'wink', 'spin', 'bright', 'squint', 'scan']) {
    const end = eyePose(0, action, 1, gaze);
    assert.ok(Math.abs(end.x - gaze.x) < 1e-10);
    assert.ok(Math.abs(end.y - gaze.y) < 1e-10);
    assert.ok(Math.abs(end.roll) < 1e-10);
  }
});

test('gaze spring settles consistently across display frame rates', () => {
  for (const fps of [30, 60, 144]) {
    let value = 0, velocity = 0;
    for (let i = 0; i < fps; i++) ({ value, velocity } = springStep(value, velocity, 10, 1 / fps));
    assert.ok(Math.abs(value - 10) < 0.001);
    assert.ok(Math.abs(velocity) < 0.01);
  }
});


test('blinks close fully, hold briefly and reopen more slowly', () => {
  assert.equal(blinkLid(0), 1);
  assert.equal(blinkLid(0.3), 0.02);
  assert.equal(blinkLid(0.4), 0.02);
  assert.ok(blinkLid(0.65) < 0.5);
  assert.equal(blinkLid(1), 1);
  assert.equal(blinkLid(2), 1);
});

test('new expressions visibly change eye shape and release it at the end', () => {
  const gaze = { x: 0, y: 0 };
  assert.ok(eyePose(0, 'bright', 0.35, gaze).scaleY > 1.2);
  assert.ok(eyePose(0, 'squint', 0.35, gaze).scaleY < 0.5);
  for (const action of ['bright', 'squint', 'scan']) {
    assert.equal(eyePose(0, action, 1, gaze).scaleY, 1);
    assert.equal(eyePose(0, action, 1, gaze).scaleX, 1);
  }
});
test('idle hops crouch, leave the ground and land back at rest', () => {
  const rest = { rise: 0, scaleX: 1, scaleY: 1, tilt: 0 };
  assert.deepEqual(hopPose(0), rest);
  assert.deepEqual(hopPose(1), rest);
  assert.ok(hopPose(0.2).scaleY < 0.9);
  const air = hopPose(0.5);
  assert.ok(air.rise > 0.9 && air.scaleY > 1);
  assert.ok(hopPose(0.84).scaleY < 1);
  for (const joint of [0.22, 0.3, 0.78, 0.9]) {
    const before = hopPose(joint - 1e-6), after = hopPose(joint + 1e-6);
    assert.ok(Math.abs(before.rise - after.rise) < 1e-3);
    assert.ok(Math.abs(before.scaleY - after.scaleY) < 1e-3);
  }
});

test('wiggles rock side to side and die out at both ends', () => {
  assert.equal(wigglePose(0).tilt, 0);
  assert.equal(wigglePose(1).tilt, 0);
  assert.ok(wigglePose(0.1).tilt > 1);
  assert.ok(wigglePose(0.3).tilt < -1);
  assert.ok(Math.abs(wigglePose(0.98).tilt) < 1);
});

test('the split entrance pops companions onto mirrored perches without a slow fade', () => {
  assert.equal(splitEnter(0, 1).opacity, 0);
  assert.equal(splitEnter(0, 2).opacity, 0);
  assert.ok(Math.abs(splitRest(1).x + splitRest(2).x) < 1e-10);
  for (const slot of [1, 2]) {
    const out = splitEnter(1, slot), rest = splitRest(slot);
    assert.ok(Math.abs(out.x - rest.x) < 1e-10 && Math.abs(out.y - rest.y) < 1e-10);
    assert.ok(Math.abs(out.scale - rest.scale) < 1e-10);
    assert.equal(out.opacity, 1);
  }
  assert.ok(splitEnter(0.2, 1).opacity >= 1);
  assert.ok(splitEnter(0.65, 1).scale > splitRest(1).scale);
  const main = splitEnter(1, 0);
  assert.ok(Math.abs(main.y - splitRest(0).y) < 1e-10);
  assert.equal(main.scaleY, 1);
  assert.ok(splitEnter(0.12, 0).scaleY < 1);
});

test('the split exit returns everyone to one resting ball', () => {
  for (const slot of [1, 2]) {
    const start = splitExit(0, slot), end = splitExit(1, slot);
    assert.ok(Math.abs(start.x - splitRest(slot).x) < 1e-10);
    assert.ok(Math.abs(start.y - splitRest(slot).y) < 1e-10);
    assert.ok(Math.abs(end.x) < 1e-10 && Math.abs(end.y) < 1e-10);
    assert.equal(end.opacity, 0);
  }
  const mainEnd = splitExit(1, 0);
  assert.equal(mainEnd.y, 0);
  assert.equal(mainEnd.scaleY, 1);
  assert.ok(splitExit(0.78, 0).scaleY < 1);
});

test('the new expressions visibly move the face and release it at the end', () => {
  const gaze = { x: 2, y: -1 };
  assert.ok(eyePose(0, 'think', 0.35, gaze).y < gaze.y - 5);
  assert.ok(eyePose(0, 'shy', 0.35, gaze).y > gaze.y + 5);
  const dizzy = eyePose(0, 'dizzy', 0.35, gaze);
  assert.ok(Math.abs(dizzy.x - gaze.x) > 0.5 || Math.abs(dizzy.roll) > 0.5);
  for (const action of ['think', 'shy', 'dizzy']) {
    const end = eyePose(0, action, 1, gaze);
    assert.ok(Math.abs(end.x - gaze.x) < 1e-10);
    assert.ok(Math.abs(end.y - gaze.y) < 1e-10);
    assert.ok(Math.abs(end.roll) < 1e-10);
    assert.equal(end.scaleY, 1);
  }
});

