import { useState } from "react";
import { createLazyFileRoute, useNavigate } from "@tanstack/react-router";
import { FiUser, FiMail, FiLock, FiShield, FiImage, FiX } from "react-icons/fi";
import Spinner from "@/components/Spinner";
import ErrorWarning from "@/components/ErrorWarning";

const MAX_FILE_SIZE = 10 * 1024 * 1024;
const ALLOWED_FILE_TYPES = ["image/jpeg", "image/png", "image/gif"];

export const Route = createLazyFileRoute("/settings/users/@create")({
  component: CreateUserPage,
});

function CreateUserPage() {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [isErrorVisible, setIsErrorVisible] = useState(false);
  const navigate = useNavigate();

  const validateFile = (file: File): string | null => {
    if (file.size > MAX_FILE_SIZE) {
      return "File size too large. Maximum size is 10MB";
    }
    if (!ALLOWED_FILE_TYPES.includes(file.type)) {
      return "Invalid file type. Allowed types: JPG, PNG, GIF";
    }
    return null;
  };

  const handleImageChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];

    if (file) {
      const error = validateFile(file);
      if (error) {
        setError(error);
        return;
      }

      if (previewUrl && previewUrl.startsWith("blob:")) {
        URL.revokeObjectURL(previewUrl);
      }
      setSelectedFile(file);
      const url = URL.createObjectURL(file);
      setPreviewUrl(url);
    }
  };

  const removeImage = () => {
    if (previewUrl && previewUrl.startsWith("blob:")) {
      URL.revokeObjectURL(previewUrl);
    }
    setPreviewUrl(null);
    setSelectedFile(null);
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");
    setIsErrorVisible(false);

    try {
      const formData = new FormData(e.currentTarget);
      let photoUrl = "";

      if (selectedFile) {
        const photoFormData = new FormData();
        photoFormData.append("photo", selectedFile);

        const photoRes = await fetch("/api/v1/users/upload-photo", {
          method: "POST",
          body: photoFormData,
        });

        if (!photoRes.ok) {
          throw new Error("Failed to upload profile photo");
        }

        const photoData = await photoRes.json();
        photoUrl = photoData.path;
      }

      const userData = {
        name: formData.get("name") as string,
        username: formData.get("username") as string,
        email: formData.get("email") as string,
        password: formData.get("password") as string,
        isAdmin: formData.get("isAdmin") === "on",
        isActive: true,
        thumb: photoUrl,
      };

      const res = await fetch("/api/v1/users/create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(userData),
      });

      if (!res.ok) {
        const errorData = await res.json();
        throw new Error(errorData.error || "Failed to create user");
      }

      navigate({
        to: "/settings/users",
        search: { page: 1, limit: 10 },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create user");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <main className='container mx-auto px-4 py-8'>
      <section className='max-w-2xl mx-auto'>
        <div className='bg-slate-800/50 backdrop-blur-sm rounded-2xl p-8 shadow-lg'>
          <header className='mb-8 flex items-center justify-between'>
            <div>
              <h1 className='text-2xl font-bold text-white'>Create New User</h1>
              <p className='mt-2 text-sky-200'>
                Fill in the information to create a new user account.
              </p>
            </div>
            {isLoading && <Spinner size='lg' />}
          </header>

          <ErrorWarning error={error} isVisible={isErrorVisible} />

          <form className='space-y-6' onSubmit={handleSubmit}>
            {/* Name Field */}
            <div>
              <label
                htmlFor='name'
                className='block text-sm font-medium text-sky-200 mb-2'
              >
                Full Name
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiUser
                    className='h-5 w-5 text-sky-400/50'
                    aria-hidden='true'
                  />
                </div>
                <input
                  type='text'
                  id='name'
                  name='name'
                  autoFocus
                  disabled={isLoading}
                  className='block w-full pl-10 pr-3 py-2.5 bg-slate-900/50 border border-sky-200/10 rounded-lg 
                           text-white placeholder:text-sky-200/50 
                           focus:outline-none focus:ring-2 focus:ring-sky-500/40 focus:border-transparent
                           transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
                  placeholder='John Doe'
                  required
                  maxLength={60}
                />
              </div>
            </div>

            {/* Username Field */}
            <div>
              <label
                htmlFor='username'
                className='block text-sm font-medium text-sky-200 mb-2'
              >
                Username
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <span className='text-sky-400/50'>@</span>
                </div>
                <input
                  type='text'
                  id='username'
                  name='username'
                  disabled={isLoading}
                  className='block w-full pl-10 pr-3 py-2.5 bg-slate-900/50 border border-sky-200/10 rounded-lg 
                           text-white placeholder:text-sky-200/50 
                           focus:outline-none focus:ring-2 focus:ring-sky-500/40 focus:border-transparent
                           transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
                  placeholder='johndoe'
                  required
                  maxLength={20}
                />
              </div>
            </div>

            {/* Email Field */}
            <div>
              <label
                htmlFor='email'
                className='block text-sm font-medium text-sky-200 mb-2'
              >
                Email Address
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiMail
                    className='h-5 w-5 text-sky-400/50'
                    aria-hidden='true'
                  />
                </div>
                <input
                  type='email'
                  id='email'
                  name='email'
                  disabled={isLoading}
                  className='block w-full pl-10 pr-3 py-2.5 bg-slate-900/50 border border-sky-200/10 rounded-lg 
                           text-white placeholder:text-sky-200/50 
                           focus:outline-none focus:ring-2 focus:ring-sky-500/40 focus:border-transparent
                           transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
                  placeholder='john@example.com'
                  required
                />
              </div>
            </div>

            {/* Password Field */}
            <div>
              <label
                htmlFor='password'
                className='block text-sm font-medium text-sky-200 mb-2'
              >
                Password
              </label>
              <div className='relative'>
                <div className='absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none'>
                  <FiLock
                    className='h-5 w-5 text-sky-400/50'
                    aria-hidden='true'
                  />
                </div>
                <input
                  type='password'
                  id='password'
                  name='password'
                  disabled={isLoading}
                  className='block w-full pl-10 pr-3 py-2.5 bg-slate-900/50 border border-sky-200/10 rounded-lg 
                           text-white placeholder:text-sky-200/50 
                           focus:outline-none focus:ring-2 focus:ring-sky-500/40 focus:border-transparent
                           transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
                  placeholder='••••••••'
                  required
                  minLength={9}
                  maxLength={128}
                />
              </div>
            </div>

            {/* Admin Toggle */}
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-3'>
                <FiShield className='h-5 w-5 text-sky-400/50' />
                <div>
                  <label
                    htmlFor='isAdmin'
                    className='text-sm font-medium text-sky-200'
                  >
                    Admin User
                  </label>
                  <p className='text-xs text-sky-200/70'>
                    Grant administrative privileges to this user
                  </p>
                </div>
              </div>
              <label className='relative inline-flex items-center cursor-pointer'>
                <input
                  type='checkbox'
                  name='isAdmin'
                  id='isAdmin'
                  disabled={isLoading}
                  className='sr-only peer'
                />
                <div
                  className="w-11 h-6 bg-slate-900/50 peer-focus:outline-none 
                            peer-focus:ring-2 peer-focus:ring-sky-500/40
                            rounded-full peer peer-checked:after:translate-x-full 
                            peer-checked:after:border-white after:content-[''] 
                            after:absolute after:top-[2px] after:left-[2px] 
                            after:bg-white after:border-gray-300 after:border 
                            after:rounded-full after:h-5 after:w-5 
                            after:transition-all peer-checked:bg-sky-500/50"
                ></div>
              </label>
            </div>

            <div className='pt-4'>
              <button
                type='submit'
                disabled={isLoading}
                className='w-full px-4 py-2.5 bg-sky-500/50 hover:bg-sky-500/70 
                         text-white font-medium rounded-lg transition-colors
                         disabled:opacity-50 disabled:cursor-not-allowed
                         focus:outline-none focus:ring-2 focus:ring-sky-500/40'
              >
                {isLoading ? "Creating..." : "Create User"}
              </button>
            </div>
          </form>
        </div>
      </section>
    </main>
  );
}
