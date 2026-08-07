import { useEffect, useRef } from "react";

const POSITION_VERTEX_SHADER = `
  attribute vec2 a_position;
  attribute vec2 a_texCoord;
  varying vec2 v_texCoord;

  void main() {
    gl_Position = vec4(a_position, 0.0, 1.0);
    v_texCoord = a_texCoord;
  }
`;

const FIELD_DITHER_FRAGMENT_SHADER = `
  precision highp float;

  uniform sampler2D u_image;
  uniform vec2 u_resolution;
  varying vec2 v_texCoord;

  float orderedThreshold(vec2 position) {
    int x = int(mod(position.x, 4.0));
    int y = int(mod(position.y, 4.0));
    int index = y * 4 + x;
    float thresholds[16];
    thresholds[0] = 0.0;
    thresholds[1] = 12.0;
    thresholds[2] = 3.0;
    thresholds[3] = 15.0;
    thresholds[4] = 8.0;
    thresholds[5] = 4.0;
    thresholds[6] = 11.0;
    thresholds[7] = 7.0;
    thresholds[8] = 2.0;
    thresholds[9] = 14.0;
    thresholds[10] = 1.0;
    thresholds[11] = 13.0;
    thresholds[12] = 10.0;
    thresholds[13] = 6.0;
    thresholds[14] = 9.0;
    thresholds[15] = 5.0;

    for (int current = 0; current < 16; current++) {
      if (current == index) {
        return thresholds[current] / 16.0;
      }
    }

    return 0.0;
  }

  void main() {
    vec3 source = texture2D(u_image, v_texCoord).rgb;
    float gray = dot(source, vec3(0.299, 0.587, 0.114));
    gray = clamp(gray * 1.22 - 0.12, 0.0, 1.0);

    float threshold = orderedThreshold(gl_FragCoord.xy);
    float mark = step(threshold, gray);
    vec3 paper = vec3(0.973, 0.965, 0.902);
    vec3 moss = vec3(0.16, 0.32, 0.15);
    vec3 leaf = vec3(0.69, 0.76, 0.31);
    vec3 ink = mix(moss, leaf, smoothstep(0.42, 0.88, gray));

    gl_FragColor = vec4(mix(paper, ink, mark), 1.0);
  }
`;

type FieldSource = HTMLVideoElement | HTMLImageElement;

function createShader(
  gl: WebGLRenderingContext,
  type: number,
  source: string,
): WebGLShader | null {
  const shader = gl.createShader(type);
  if (shader === null) {
    return null;
  }

  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    return shader;
  }

  gl.deleteShader(shader);
  return null;
}

function createProgram(gl: WebGLRenderingContext): WebGLProgram | null {
  const vertexShader = createShader(gl, gl.VERTEX_SHADER, POSITION_VERTEX_SHADER);
  const fragmentShader = createShader(
    gl,
    gl.FRAGMENT_SHADER,
    FIELD_DITHER_FRAGMENT_SHADER,
  );

  if (vertexShader === null || fragmentShader === null) {
    if (vertexShader !== null) gl.deleteShader(vertexShader);
    if (fragmentShader !== null) gl.deleteShader(fragmentShader);
    return null;
  }

  const program = gl.createProgram();
  if (program === null) {
    gl.deleteShader(vertexShader);
    gl.deleteShader(fragmentShader);
    return null;
  }

  gl.attachShader(program, vertexShader);
  gl.attachShader(program, fragmentShader);
  gl.linkProgram(program);
  gl.deleteShader(vertexShader);
  gl.deleteShader(fragmentShader);

  if (gl.getProgramParameter(program, gl.LINK_STATUS)) {
    return program;
  }

  gl.deleteProgram(program);
  return null;
}

function sourceSize(source: FieldSource): Readonly<{ width: number; height: number }> {
  if (source instanceof HTMLVideoElement) {
    return { width: source.videoWidth, height: source.videoHeight };
  }

  return { width: source.naturalWidth, height: source.naturalHeight };
}

/** Renders local wheat-field footage through a Garden-colored dither shader. */
export function DitheredField() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const video = videoRef.current;
    if (canvas === null || video === null) {
      return;
    }

    const gl = canvas.getContext("webgl", {
      alpha: false,
      antialias: false,
      premultipliedAlpha: false,
      preserveDrawingBuffer: true,
    });
    if (gl === null) {
      return;
    }

    const program = createProgram(gl);
    const positionBuffer = gl.createBuffer();
    const textureCoordinateBuffer = gl.createBuffer();
    const texture = gl.createTexture();
    if (
      program === null ||
      positionBuffer === null ||
      textureCoordinateBuffer === null ||
      texture === null
    ) {
      if (program !== null) gl.deleteProgram(program);
      if (positionBuffer !== null) gl.deleteBuffer(positionBuffer);
      if (textureCoordinateBuffer !== null) gl.deleteBuffer(textureCoordinateBuffer);
      if (texture !== null) gl.deleteTexture(texture);
      return;
    }

    const positionLocation = gl.getAttribLocation(program, "a_position");
    const textureCoordinateLocation = gl.getAttribLocation(program, "a_texCoord");
    const resolutionLocation = gl.getUniformLocation(program, "u_resolution");
    const imageLocation = gl.getUniformLocation(program, "u_image");

    gl.useProgram(program);
    gl.bindBuffer(gl.ARRAY_BUFFER, positionBuffer);
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
      gl.STATIC_DRAW,
    );
    gl.enableVertexAttribArray(positionLocation);
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0);

    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.uniform1i(imageLocation, 0);

    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const fallbackImage = new Image();
    fallbackImage.src = "/media/field-wind-poster.jpg";

    let currentSource: FieldSource | null = null;
    let videoFrameId: number | null = null;
    let playRequest: Promise<void> | null = null;
    let isVisible = true;
    let isDisposed = false;

    const configureCanvas = (source: FieldSource) => {
      const dimensions = sourceSize(source);
      if (dimensions.width === 0 || dimensions.height === 0) {
        return false;
      }

      const displayWidth = canvas.offsetWidth;
      const displayHeight = canvas.offsetHeight;
      if (displayWidth === 0 || displayHeight === 0) {
        return false;
      }

      canvas.width = Math.max(1, Math.round(displayWidth));
      canvas.height = Math.max(1, Math.round(displayHeight));
      gl.viewport(0, 0, canvas.width, canvas.height);
      gl.uniform2f(resolutionLocation, canvas.width, canvas.height);

      const canvasAspect = canvas.width / canvas.height;
      const sourceAspect = dimensions.width / dimensions.height;
      let left = 0;
      let right = 1;
      let top = 1;
      let bottom = 0;

      if (sourceAspect > canvasAspect) {
        const visibleWidth = canvasAspect / sourceAspect;
        left = (1 - visibleWidth) / 2;
        right = 1 - left;
      } else {
        const visibleHeight = sourceAspect / canvasAspect;
        bottom = (1 - visibleHeight) / 2;
        top = 1 - bottom;
      }

      gl.bindBuffer(gl.ARRAY_BUFFER, textureCoordinateBuffer);
      gl.bufferData(
        gl.ARRAY_BUFFER,
        new Float32Array([left, bottom, right, bottom, left, top, left, top, right, bottom, right, top]),
        gl.STATIC_DRAW,
      );
      gl.enableVertexAttribArray(textureCoordinateLocation);
      gl.vertexAttribPointer(textureCoordinateLocation, 2, gl.FLOAT, false, 0, 0);
      currentSource = source;
      return true;
    };

    const render = () => {
      const source = currentSource;
      if (source === null || isDisposed) {
        return;
      }

      gl.bindTexture(gl.TEXTURE_2D, texture);
      gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 1);
      gl.texImage2D(
        gl.TEXTURE_2D,
        0,
        gl.RGBA,
        gl.RGBA,
        gl.UNSIGNED_BYTE,
        source,
      );
      gl.drawArrays(gl.TRIANGLES, 0, 6);
      canvas.dataset.renderState = "ready";
    };

    const shouldRenderVideo = () =>
      currentSource === video &&
      !video.paused &&
      isVisible &&
      !reducedMotion.matches &&
      !isDisposed;

    const cancelVideoFrame = () => {
      if (videoFrameId === null) return;
      video.cancelVideoFrameCallback(videoFrameId);
      videoFrameId = null;
    };

    const scheduleVideoFrame = () => {
      if (videoFrameId !== null || !shouldRenderVideo()) return;

      videoFrameId = video.requestVideoFrameCallback(() => {
        videoFrameId = null;
        if (!shouldRenderVideo()) return;
        render();
        scheduleVideoFrame();
      });
    };

    const startVideo = () => {
      if (isDisposed || !isVisible || reducedMotion.matches) return;

      if (!video.paused) {
        if (currentSource !== video && configureCanvas(video)) render();
        scheduleVideoFrame();
        return;
      }

      if (playRequest !== null) return;
      playRequest = video.play();
      void playRequest.then(
        () => {
          playRequest = null;
          if (isDisposed) return;
          if (configureCanvas(video)) render();
          scheduleVideoFrame();
        },
        () => {
          playRequest = null;
          if (fallbackImage.complete && configureCanvas(fallbackImage)) render();
        },
      );
    };

    const showFallback = () => {
      cancelVideoFrame();
      video.pause();
      if (fallbackImage.complete && configureCanvas(fallbackImage)) {
        render();
      }
    };

    const handleMotionChange = () => {
      if (reducedMotion.matches) showFallback();
      else startVideo();
    };
    const handleResize = () => {
      if (currentSource !== null && configureCanvas(currentSource)) render();
    };
    const handleVideoReady = () => startVideo();
    const handleInteraction = () => startVideo();
    const handleFallbackReady = () => {
      if (currentSource === null || reducedMotion.matches) showFallback();
    };

    const resizeObserver = new ResizeObserver(handleResize);
    const intersectionObserver = new IntersectionObserver(
      ([entry]) => {
        isVisible = entry.isIntersecting;
        if (isVisible) startVideo();
        else {
          cancelVideoFrame();
          video.pause();
        }
      },
      { rootMargin: "160px 0px" },
    );

    video.addEventListener("loadeddata", handleVideoReady);
    fallbackImage.addEventListener("load", handleFallbackReady);
    document.addEventListener("click", handleInteraction, { once: true });
    document.addEventListener("touchstart", handleInteraction, { once: true });
    document.addEventListener("keydown", handleInteraction, { once: true });
    reducedMotion.addEventListener("change", handleMotionChange);
    resizeObserver.observe(canvas);
    intersectionObserver.observe(canvas);

    if (fallbackImage.complete) handleFallbackReady();
    if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) handleVideoReady();

    return () => {
      isDisposed = true;
      cancelVideoFrame();
      video.pause();
      video.removeEventListener("loadeddata", handleVideoReady);
      fallbackImage.removeEventListener("load", handleFallbackReady);
      document.removeEventListener("click", handleInteraction);
      document.removeEventListener("touchstart", handleInteraction);
      document.removeEventListener("keydown", handleInteraction);
      reducedMotion.removeEventListener("change", handleMotionChange);
      resizeObserver.disconnect();
      intersectionObserver.disconnect();
      delete canvas.dataset.renderState;
      gl.deleteTexture(texture);
      gl.deleteBuffer(positionBuffer);
      gl.deleteBuffer(textureCoordinateBuffer);
      gl.deleteProgram(program);
    };
  }, []);

  return (
    <div className="hero-field" aria-hidden="true">
      <canvas ref={canvasRef} />
      <video ref={videoRef} muted loop playsInline preload="metadata" tabIndex={-1}>
        <source src="/media/field-wind.webm" type="video/webm" />
        <source src="/media/field-wind.mp4" type="video/mp4" />
      </video>
    </div>
  );
}
