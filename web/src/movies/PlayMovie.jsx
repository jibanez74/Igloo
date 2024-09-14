import React from 'react';
import VideoPlayer from '../shared/VideoPlayer';

function PlayMovie({ movieId }) {
  const options = {
    autoplay: true,
    controls: true,
    responsive: true,
    fluid: true,
    sources: [{
      src: `/api/stream/${movieId}`,
      type: 'application/x-mpegURL'
    }]
  };

  return (
    <div>
      <h1>Now Playing</h1>
      <VideoPlayer options={options} />
    </div>
  );
}

export default PlayMovie;
