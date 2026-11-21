"use client";

import { useEffect, useRef, useState } from "react";
import * as THREE from "three";

type TenantStatus = "connected" | "pairing" | "disconnected" | "logged_out";

const COLORS: Record<TenantStatus, string> = {
  connected: "#25d366",
  pairing: "#8a9225",
  disconnected: "#8b95a5",
  logged_out: "#ef4444",
};

const THREE_COLORS: Record<TenantStatus, number> = {
  connected: 0x25d366,
  pairing: 0x8a9225,
  disconnected: 0x8b95a5,
  logged_out: 0xef4444,
};

export function StatusOrb({
  status,
  size = 88,
  className,
}: {
  status: TenantStatus;
  size?: number;
  className?: string;
}) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const [webglFailed, setWebglFailed] = useState(false);
  const color = COLORS[status];

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    let disposed = false;
    let frame = 0;
    let cleanup = () => {};
    let visible = !document.hidden;

    const start = () => {
      if (disposed || !hostRef.current) return;

      let renderer: THREE.WebGLRenderer;
      try {
        renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: "low-power" });
      } catch {
        setWebglFailed(true);
        return;
      }

      const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      const scene = new THREE.Scene();
      const camera = new THREE.PerspectiveCamera(40, 1, 0.1, 100);
      camera.position.set(0, 0, 4.6);

      const group = new THREE.Group();
      scene.add(group);

      const coreGeometry = new THREE.IcosahedronGeometry(1.08, 0);
      const coreMaterial = new THREE.MeshBasicMaterial({
        color: THREE_COLORS[status],
        wireframe: true,
        transparent: true,
        opacity: 0.9,
      });
      const core = new THREE.Mesh(coreGeometry, coreMaterial);

      const shellGeometry = new THREE.IcosahedronGeometry(1.66, 0);
      const shellMaterial = new THREE.MeshBasicMaterial({
        color: 0x6e66d8,
        wireframe: true,
        transparent: true,
        opacity: 0.32,
      });
      const shell = new THREE.Mesh(shellGeometry, shellMaterial);
      group.add(core, shell);

      const count = 90;
      const positions = new Float32Array(count * 3);
      for (let i = 0; i < count; i += 1) {
        const radius = 1.85 + Math.random() * 0.55;
        const theta = Math.random() * Math.PI * 2;
        const phi = Math.acos(2 * Math.random() - 1);
        positions[i * 3] = radius * Math.sin(phi) * Math.cos(theta);
        positions[i * 3 + 1] = radius * Math.sin(phi) * Math.sin(theta);
        positions[i * 3 + 2] = radius * Math.cos(phi);
      }
      const pointsGeometry = new THREE.BufferGeometry();
      pointsGeometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
      const pointsMaterial = new THREE.PointsMaterial({
        color: 0x8f88f5,
        size: 0.03,
        transparent: true,
        opacity: 0.45,
        sizeAttenuation: true,
      });
      const points = new THREE.Points(pointsGeometry, pointsMaterial);
      group.add(points);

      renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
      renderer.setSize(size, size, false);
      renderer.setClearColor(0x000000, 0);
      const canvas = renderer.domElement;
      canvas.style.width = `${size}px`;
      canvas.style.height = `${size}px`;
      canvas.style.display = "block";
      host.appendChild(canvas);

      const clock = new THREE.Clock();
      const renderFrame = () => {
        if (!visible || disposed) return;
        const elapsed = clock.getElapsedTime();
        if (!reduceMotion) {
          group.rotation.y = elapsed * 0.32;
          group.rotation.x = Math.sin(elapsed * 0.55) * 0.12;
          shell.rotation.y = -elapsed * 0.18;
          points.rotation.y = elapsed * 0.08;
          const pulse = status === "connected" ? 1 + Math.sin(elapsed * 2.1) * 0.035 : 1;
          group.scale.setScalar(pulse);
        }
        renderer.render(scene, camera);
        if (!reduceMotion) {
          frame = requestAnimationFrame(renderFrame);
        }
      };
      renderFrame();

      const onVisibility = () => {
        visible = !document.hidden;
        if (visible && !reduceMotion) {
          clock.start();
          frame = requestAnimationFrame(renderFrame);
        } else {
          cancelAnimationFrame(frame);
        }
      };
      document.addEventListener("visibilitychange", onVisibility);

      cleanup = () => {
        document.removeEventListener("visibilitychange", onVisibility);
        cancelAnimationFrame(frame);
        coreGeometry.dispose();
        coreMaterial.dispose();
        shellGeometry.dispose();
        shellMaterial.dispose();
        pointsGeometry.dispose();
        pointsMaterial.dispose();
        renderer.dispose();
        if (canvas.parentNode === host) {
          host.removeChild(canvas);
        }
      };
    };

    start();
    return () => {
      disposed = true;
      cleanup();
      host.replaceChildren();
    };
  }, [status, size]);

  if (webglFailed) {
    return (
      <div
        className={`relative grid place-items-center ${className ?? ""}`}
        style={{ width: size, height: size }}
        role="img"
        aria-label={`Sessão ${status}`}
      >
        <span
          className="absolute inset-2 rounded-full border"
          style={{ borderColor: color, boxShadow: `inset 0 0 24px ${color}22` }}
        />
        <span className="size-2 rounded-full" style={{ backgroundColor: color }} />
      </div>
    );
  }

  return (
    <div
      ref={hostRef}
      className={className}
      style={{ width: size, height: size }}
      role="img"
      aria-label={`Sessão ${status}`}
    />
  );
}
