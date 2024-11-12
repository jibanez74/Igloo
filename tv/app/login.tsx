import { useState } from "react";
import {
  View,
  StyleSheet,
  Image,
  KeyboardAvoidingView,
  AccessibilityInfo,
} from "react-native";
import {
  Snackbar,
  TextInput,
  Button,
  IconButton,
} from "react-native-paper";
import api from "@/lib/api";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "@/components/ThemedView";
import { ThemedText } from "@/components/ThemedText";

export default function LoginScreen() {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [focusedInput, setFocusedInput] = useState<string | null>(null);

  const handleLogin = async () => {
    setLoading(true);
    setError(null);

    try {
      const res = await api.post("/login", {
        username,
        email,
        password,
      });

      console.log(res.data);
    } catch (err) {
      console.error(err);
      setError("An error occurred. Please try again.");
      AccessibilityInfo.announceForAccessibility("Login failed. An error occurred. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  const getFocusedStyle = (isFocused: boolean) =>
    isFocused
      ? {
          borderColor: Colors.secondary,
          transform: [{ scale: 1.05 }],
        }
      : {};

  return (
    <ThemedView variant="dark" style={styles.container}>
      <View style={styles.content}>
        <Image
          source={require("../assets/images/logo-alt.png")}
          style={styles.logo}
          accessible={true}
          accessibilityLabel="Igloo Logo"
          accessibilityRole="image"
        />

        <ThemedText
          variant="light"
          size="xlarge"
          weight="bold"
          style={styles.welcomeText}
          accessible={true}
          accessibilityRole="header"
        >
          Welcome to Igloo
        </ThemedText>

        <TextInput
          label="Username"
          value={username}
          onChangeText={setUsername}
          autoCapitalize="none"
          autoCorrect={false}
          autoFocus={true}
          maxLength={20}
          style={[styles.input, getFocusedStyle(focusedInput === "username")]}
          mode="outlined"
          disabled={loading}
          tvParallaxProperties={{ enabled: false }}
          nextFocusDown={1}
          hasTVPreferredFocus={true}
          onFocus={() => setFocusedInput("username")}
          onBlur={() => setFocusedInput(null)}
          theme={{ colors: { primary: Colors.secondary }}}
          outlineColor={Colors.info}
          textColor={Colors.light}
        />

        <TextInput
          label="Email"
          value={email}
          onChangeText={setEmail}
          autoCapitalize="none"
          autoCorrect={false}
          style={[styles.input, getFocusedStyle(focusedInput === "email")]}
          mode="outlined"
          keyboardType="email-address"
          disabled={loading}
          nextFocusUp={0}
          nextFocusDown={2}
          onFocus={() => setFocusedInput("email")}
          onBlur={() => setFocusedInput(null)}
          theme={{ colors: { primary: Colors.secondary }}}
          outlineColor={Colors.info}
          textColor={Colors.light}
        />

        <View style={styles.passwordContainer}>
          <TextInput
            label="Password"
            value={password}
            onChangeText={setPassword}
            autoCapitalize="none"
            autoCorrect={false}
            maxLength={128}
            style={[styles.input, styles.passwordInput, getFocusedStyle(focusedInput === "password")]}
            mode="outlined"
            secureTextEntry={!showPassword}
            disabled={loading}
            nextFocusUp={1}
            nextFocusDown={3}
            onFocus={() => setFocusedInput("password")}
            onBlur={() => setFocusedInput(null)}
            theme={{ colors: { primary: Colors.secondary }}}
            outlineColor={Colors.info}
            textColor={Colors.light}
          />

          <IconButton
            icon={showPassword ? "eye-off" : "eye"}
            onPress={() => setShowPassword(prev => !prev)}
            style={styles.eyeButton}
            size={32}
            iconColor={Colors.light}
          />
        </View>

        <Button
          mode="contained"
          onPress={handleLogin}
          style={styles.button}
          labelStyle={styles.buttonLabel}
          loading={loading}
          disabled={loading}
          nextFocusUp={2}
          buttonColor={Colors.secondary}
          textColor={Colors.dark}
        >
          Sign In
        </Button>
      </View>

      <Snackbar
        visible={!!error}
        onDismiss={() => setError(null)}
        duration={5000}
        style={[styles.snackbar, { backgroundColor: Colors.danger }]}
      >
        <ThemedText variant="dark" size="medium">
          {error}
        </ThemedText>
      </Snackbar>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  content: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    paddingHorizontal: 80,
    width: "100%",
  },
  logo: {
    width: 150,
    height: 150,
    marginBottom: 30,
  },
  welcomeText: {
    marginBottom: 50,
    textAlign: "center",
  },
  input: {
    width: "100%",
    marginBottom: 30,
    height: 60,
    backgroundColor: Colors.primary,
  },
  passwordContainer: {
    flexDirection: "row",
    alignItems: "center",
    width: "100%",
  },
  passwordInput: {
    flex: 1,
  },
  eyeButton: {
    marginLeft: 10,
    backgroundColor: Colors.primary,
  },
  button: {
    marginTop: 40,
    width: "100%",
    paddingVertical: 15,
  },
  buttonLabel: {
    fontSize: 24,
  },
  snackbar: {
    position: 'absolute',
    bottom: 50,
    width: "50%",
    alignSelf: "center",
  },
});
