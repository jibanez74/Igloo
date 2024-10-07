import { useState, useEffect } from "react";

type AlertProps = {
  title?: string;
  msg?: string;
  variant?: "danger" | "success" | "warning";
  time?: number;
};

export default function Alert({
  title = "Error",
  msg = "an error occurred",
  time = 5000,
  variant = "danger",
}: AlertProps) {
  const [show, setShow] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => setShow(false), time);

    return () => clearTimeout(timer);
  }, [time]);

  return !show ? null : (
    <div className={`border-l-4 p-4 bg-${variant} rounded-md`} role='alert'>
      <div className='font-bold'>{title}</div>
      <div>{msg}</div>
    </div>
  );
}

// ... existing component code ...
