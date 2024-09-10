import PropTypes from "prop-types";
import { useState, useEffect } from "react";

export default function Alert({ title, msg, time, variant }) {
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

Alert.propTypes = {
  title: PropTypes.string.isRequired,
  msg: PropTypes.string.isRequired,
  time: PropTypes.number.isRequired,
  variant: PropTypes.string.isRequired,
};
