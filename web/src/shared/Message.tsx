import { useState, useEffect } from "react";
import Alert from "react-bootstrap/Alert";

type MessageProps = {
  title?: string;
  msg?: string;
  variant?: "danger" | "warning" | "success" | "info" | "primary" | "secondary";
  duration?: number;
};

export default function Message({
  title = "Error",
  msg = "an error occurred",
  variant = "danger",
  duration = 5000,
}: MessageProps) {
  const [show, setShow] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => setShow(false), duration);

    return () => clearTimeout(timer);
  }, [duration]);

  return !show ? null : (
    <Alert
      dismissible
      onClose={() => setShow(false)}
      variant={variant}
      role='alert'
      aria-live='polite'
    >
      <Alert.Heading>{title}</Alert.Heading>
      <p className='mb-0'>{msg}</p>
    </Alert>
  );
}
