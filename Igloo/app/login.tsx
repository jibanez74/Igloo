import {
  StyleSheet,
  View,
  Text,
  TVFocusGuideView,
  Pressable,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import Colors from "@/constants/Colors";

export default function LoginScreen() {
  const [focusedInput, setFocusedInput] = useState<string | null>(null);
  const { signIn } = useAuth();

  const handleLogin = async () => {
    try {
      await signIn("username", "password");
    } catch (error) {
      console.error("Login failed:", error);
    }
  };

  return (
    <SafeAreaView style={styles.container}>
      <TVFocusGuideView style={styles.loginContainer}>
        <Text style={styles.title}>Welcome</Text>

        <Pressable
          focusable
          style={[
            styles.input,
            focusedInput === "username" && styles.inputFocused
          ]}
          onFocus={() => setFocusedInput("username")}
          onBlur={() => setFocusedInput(null)}
        >
          <Text 
            style={[
              styles.inputText,
              focusedInput === "username" && styles.textFocused
            ]}
          >
            Enter Username
          </Text>
        </Pressable>

        <Pressable
          focusable
          style={[
            styles.input,
            focusedInput === "password" && styles.inputFocused
          ]}
          onFocus={() => setFocusedInput("password")}
          onBlur={() => setFocusedInput(null)}
        >
          <Text 
            style={[
              styles.inputText,
              focusedInput === "password" && styles.textFocused
            ]}
          >
            Enter Password
          </Text>
        </Pressable>

        <Pressable
          focusable
          style={[
            styles.loginButton,
            focusedInput === "login" && styles.buttonFocused,
          ]}
          onFocus={() => setFocusedInput("login")}
          onBlur={() => setFocusedInput(null)}
          onPress={handleLogin}
        >
          <Text 
            style={[
              styles.buttonText,
              focusedInput === "login" && styles.textFocused
            ]}
          >
            Login
          </Text>
        </Pressable>
      </TVFocusGuideView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  loginContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    padding: 40,
  },
  title: {
    fontSize: 64,
    color: Colors.textPrimary,
    marginBottom: 40,
    fontWeight: "bold",
  },
  input: {
    width: "50%",
    padding: 20,
    marginVertical: 10,
    backgroundColor: Colors.cardBackground,
    borderRadius: 10,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  inputFocused: {
    backgroundColor: Colors.secondary,
    transform: [{ scale: 1.1 }],
    borderColor: Colors.accent,
  },
  inputText: {
    color: Colors.textSecondary,
    fontSize: 24,
  },
  loginButton: {
    marginTop: 20,
    padding: 20,
    backgroundColor: Colors.accent,
    borderRadius: 10,
    width: "30%",
    borderWidth: 2,
    borderColor: 'transparent',
  },
  buttonFocused: {
    backgroundColor: Colors.secondary,
    transform: [{ scale: 1.1 }],
    borderColor: Colors.accent,
  },
  buttonText: {
    color: Colors.textSecondary,
    fontSize: 24,
    textAlign: "center",
    fontWeight: "bold",
  },
  textFocused: {
    color: Colors.textPrimary,
  },
});
