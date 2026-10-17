import { Track, type AudioProcessorOptions, type TrackProcessor } from 'livekit-client';

// Caller voice masking (F34): a delay-line pitch shifter running in an AudioWorklet. It is applied
// at capture, before the track is published, so the unmasked voice never leaves the device.
const worklet = `
class PitchShift extends AudioWorkletProcessor {
  constructor(o) {
    super();
    this.ratio = o.processorOptions.ratio;
    this.size = 2048;
    this.buf = new Float32Array(8192);
    this.w = 0;
    this.phase = 0;
  }
  read(d) {
    const n = this.buf.length;
    const pos = (this.w - d + n) % n;
    const i = Math.floor(pos), f = pos - i;
    return this.buf[i] * (1 - f) + this.buf[(i + 1) % n] * f;
  }
  process(inputs, outputs) {
    const inp = inputs[0][0], out = outputs[0][0];
    if (!inp || !out) return true;
    const step = (1 - this.ratio) / this.size;
    for (let k = 0; k < inp.length; k++) {
      this.buf[this.w] = inp[k];
      const p1 = this.phase, p2 = (this.phase + 0.5) % 1;
      out[k] = this.read(p1 * this.size + 1) * Math.sin(Math.PI * p1) + this.read(p2 * this.size + 1) * Math.sin(Math.PI * p2);
      this.phase = (this.phase + step + 1) % 1;
      this.w = (this.w + 1) % this.buf.length;
    }
    for (let c = 1; c < outputs[0].length; c++) outputs[0][c].set(out);
    return true;
  }
}
registerProcessor('mb-pitch-shift', PitchShift);
`;

export const voiceMaskSupported = typeof AudioWorkletNode !== 'undefined';

export function voiceMask(ratio = 0.78): TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
  let source: MediaStreamAudioSourceNode | undefined;
  let node: AudioWorkletNode | undefined;
  let dest: MediaStreamAudioDestinationNode | undefined;
  const p: TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> = {
    name: 'mb-voice-mask',
    async init({ track, audioContext }) {
      const url = URL.createObjectURL(new Blob([worklet], { type: 'application/javascript' }));
      await audioContext.audioWorklet.addModule(url);
      URL.revokeObjectURL(url);
      source = audioContext.createMediaStreamSource(new MediaStream([track]));
      node = new AudioWorkletNode(audioContext, 'mb-pitch-shift', { processorOptions: { ratio } });
      dest = audioContext.createMediaStreamDestination();
      source.connect(node).connect(dest);
      p.processedTrack = dest.stream.getAudioTracks()[0];
    },
    async restart(opts) {
      await p.destroy();
      await p.init(opts);
    },
    async destroy() {
      source?.disconnect();
      node?.disconnect();
      p.processedTrack = undefined;
    },
  };
  return p;
}
