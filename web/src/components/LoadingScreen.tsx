import { FiLoader } from "react-icons/fi";

type LoadingScreenProps = {
  text?: string;
};

export default function LoadingScreen({
  text = "Loading...",
}: LoadingScreenProps) {
  return (
    <main className='fixed inset-0 bg-gradient-to-br from-sky-950 via-blue-900 to-indigo-950 flex flex-col items-center justify-center'>
      <div className='flex flex-col items-center gap-6 p-8 bg-white/10 backdrop-blur-sm rounded-lg'>
        <FiLoader
          className='w-12 h-12 text-blue-200 animate-spin'
          aria-hidden='true'
        />
        <h1 className='text-2xl font-medium text-white'>{text}</h1>
        <div className='w-48 h-1 bg-blue-950/50 rounded overflow-hidden'>
          <div className='h-full bg-blue-400/50 animate-progress rounded' />
        </div>
      </div>
    </main>
  );
}
