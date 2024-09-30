import { createEffect, createSignal, onCleanup } from "solid-js";
import type { Component } from "solid-js";

type AlertProps = {
  title: string;
  msg: string;
  variant: "success" | "danger" | "info" | "warning";
  time: number;
};

const Alert: Component<AlertProps> = props => {
  const [show, setShow] = createSignal(true);
  let timer: number | null = null;

  createEffect(
    () => (timer = setTimeout(() => setShow(false), props.time || 5000)),
    [props.time]
  );

  onCleanup(() => {
    if (timer) {
      clearTimeout(timer);
    }
  });

  return !show() ? null : (
    <div
      class={`border-l-4 p-4 bg-${props.variant || "danger"} rounded-md`}
      role='alert'
    >
      <div class='font-bold'>{props.title || "Error"}</div>
      <div>{props.msg || "An error occurred"}</div>
    </div>
  );
};

export default Alert;
