import { useEffect, useState } from "react";
import { Text, View, TextInput, Pressable } from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useAuth } from "@/context/AuthContext";
import api from "@/lib/api";
import type { User } from "@/types/User";
import type { ErrorResponse } from "@/types/ErrorResponse";

type LoginResponse = {
  token: string;
  user: User;
};

export default function LoginScreen() {
  const router = useRouter();
  const { signIn, user } = useAuth();

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (user) {
      router.replace("/");
    }
  }, [user]);

  const handleLogin = async () => {
    try {
      setError("");
      setLoading(true);

      const response = await api.post<LoginResponse>("/login", {
        username,
        email,
        password,
      });

      await signIn(response.token, response.user);
    } catch (err: any) {
      console.error("Login failed:", err);
      const errorMessage =
        err.cause?.error || "Invalid credentials. Please try again.";
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  return (
    <View className='flex-1'>
      <LinearGradient
        colors={["#121F32", "#1C395E"]}
        className='flex-1 justify-center items-center'
      >
        {/* Logo */}
        <View className='mb-12'>
          <Image
            source={require("../assets/images/logo-alt.png")}
            className='w-40 h-40'
            contentFit='contain'
            transition={200}
            cachePolicy='memory-disk'
          />
        </View>

        {/* Form Container */}
        <View className='w-[35%] min-w-[400px] bg-dark/80 p-10 rounded-lg'>
          <Text className='text-3xl font-bold text-light text-center mb-8'>
            Sign In
          </Text>

          {error && (
            <Text className='text-danger text-center mb-4'>{error}</Text>
          )}

          {/* Username Input */}
          <View className='mb-6 relative'>
            <View className='absolute left-4 top-1/2 -translate-y-1/2 z-10'>
              <Ionicons
                name='person-outline'
                size={24}
                color={loading ? "#6B7280" : "#CEE3F9"}
              />
            </View>
            <TextInput
              className={`w-full h-15 bg-primary/50 text-light text-xl px-12 rounded-lg
                        ${loading ? "opacity-50" : ""}`}
              placeholder='Username'
              placeholderTextColor={loading ? "#6B7280" : "#CEE3F9"}
              autoCapitalize='none'
              autoCorrect={false}
              focusable={!loading}
              editable={!loading}
              value={username}
              onChangeText={setUsername}
            />
          </View>

          {/* Email Input */}
          <View className='mb-6 relative'>
            <View className='absolute left-4 top-1/2 -translate-y-1/2 z-10'>
              <Ionicons
                name='mail-outline'
                size={24}
                color={loading ? "#6B7280" : "#CEE3F9"}
              />
            </View>
            <TextInput
              className={`w-full h-15 bg-primary/50 text-light text-xl px-12 rounded-lg
                        ${loading ? "opacity-50" : ""}`}
              placeholder='Email'
              placeholderTextColor={loading ? "#6B7280" : "#CEE3F9"}
              autoCapitalize='none'
              autoCorrect={false}
              keyboardType='email-address'
              focusable={!loading}
              editable={!loading}
              value={email}
              onChangeText={setEmail}
            />
          </View>

          {/* Password Input */}
          <View className='mb-8 relative'>
            <View className='absolute left-4 top-1/2 -translate-y-1/2 z-10'>
              <Ionicons
                name='lock-closed-outline'
                size={24}
                color={loading ? "#6B7280" : "#CEE3F9"}
              />
            </View>
            <TextInput
              className={`w-full h-15 bg-primary/50 text-light text-xl px-12 rounded-lg
                        ${loading ? "opacity-50" : ""}`}
              placeholder='Password'
              placeholderTextColor={loading ? "#6B7280" : "#CEE3F9"}
              secureTextEntry
              autoCapitalize='none'
              autoCorrect={false}
              focusable={!loading}
              editable={!loading}
              value={password}
              onChangeText={setPassword}
            />
          </View>

          {/* Login Button */}
          <Pressable
            onPress={handleLogin}
            focusable={!loading}
            disabled={loading}
            className={`w-full bg-secondary h-15 rounded-lg justify-center items-center
                     focus:scale-105 focus:bg-info
                     ${loading ? "opacity-50" : ""}`}
          >
            <Text className='text-dark text-xl font-bold'>
              {loading ? "Signing In..." : "Sign In"}
            </Text>
          </Pressable>
        </View>
      </LinearGradient>
    </View>
  );
}
