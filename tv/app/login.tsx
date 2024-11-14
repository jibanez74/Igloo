import { useState, useRef } from "react";
import { View, StyleSheet, Image } from "react-native";
import { useTVEventHandler } from "react-native";
import {
  Snackbar,
  Text,
  TextInput,
  Button,
  IconButton,
  useTheme,
} from "react-native-paper";
import api from "@/lib/api";

export default function LoginScreen() {
  const theme = useTheme();

  const usernameRef = useRef<string>("");
  const emailRef = useRef<string>("");
  const passwordRef = useRef<string>("");

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);

  const usernameInputRef = useRef(null);
  const emailInputRef = useRef(null);
  const passwordInputRef = useRef(null);
  const loginButtonRef = useRef(null);

  useTVEventHandler((event) => {
    if (event.eventType === "right") {
      if (usernameInputRef.current?.isFocused()) {
        emailInputRef.current?.focus();
      } else if (emailInputRef.current?.isFocused()) {
        passwordInputRef.current?.focus();
      } else if (passwordInputRef.current?.isFocused()) {
        loginButtonRef.current?.focus();
      }
    } else if (event.eventType === "left") {
      if (loginButtonRef.current?.isFocused()) {
        passwordInputRef.current?.focus();
      } else if (passwordInputRef.current?.isFocused()) {
        emailInputRef.current?.focus();
      } else if (emailInputRef.current?.isFocused()) {
        usernameInputRef.current?.focus();
      }
    }
  });

  const handleLogin = async () => {
    setLoading(true);
    setError(null);

    const username = usernameRef.current;
    const email = emailRef.current;
    const password = passwordRef.current;

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
      setLoading(false);
    }
  };

  return (
    <View
      style={[styles.container, { backgroundColor: theme.colors.background }]}
    >
      <Image
        source={require("../assets/images/logo-alt.png")}
        style={styles.logo}
      />

      <Text style={styles.welcomeText}>Welcome to Igloo</Text>

      <TextInput
        ref={usernameInputRef}
        label='Username'
        defaultValue={usernameRef.current}
        onChangeText={text => (usernameRef.current = text)}
        autoCapitalize='none'
        autoCorrect={false}
        autoFocus={true}
        maxLength={20}
        style={styles.input}
        mode='outlined'
        disabled={loading}
      />

      <TextInput
        ref={emailInputRef}
        label='Email'
        defaultValue={emailRef.current}
        autoCapitalize='none'
        autoCorrect={false}
        onChangeText={text => (emailRef.current = text)}
        style={styles.input}
        mode='outlined'
        keyboardType='email-address'
        disabled={loading}
      />

      <View style={styles.passwordContainer}>
        <TextInput
          ref={passwordInputRef}
          label='Password'
          defaultValue={passwordRef.current}
          autoCapitalize='none'
          autoCorrect={false}
          maxLength={128}
          onChangeText={text => (passwordRef.current = text)}
          style={[styles.input, styles.passwordInput]}
          mode='outlined'
          secureTextEntry={!showPassword} // Controlled by showPassword state
          disabled={loading}
        />

        <IconButton
          icon={showPassword ? "eye-off" : "eye"}
          onPress={() => setShowPassword(prev => !prev)}
          style={styles.eyeButton}
        />
      </View>

      <Button
        ref={loginButtonRef}
        mode='contained'
        onPress={handleLogin}
        style={styles.button}
        labelStyle={styles.buttonLabel}
        loading={loading}
        disabled={loading}
      >
        Sign In
      </Button>

      {/* Error Snackbar */}
      <Snackbar
        visible={!!error}
        onDismiss={() => setError(null)}
        duration={3000}
        style={{ backgroundColor: theme.colors.error }}
      >
        {error}
      </Snackbar>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    paddingHorizontal: 40,
  },
  logo: {
    width: 100,
    height: 100,
    marginBottom: 20,
  },
  welcomeText: {
    fontSize: 24,
    fontWeight: "bold",
    marginBottom: 30,
    textAlign: "center",
  },
  input: {
    width: "100%",
    marginBottom: 15,
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
    marginTop: -15,
  },
  button: {
    marginTop: 20,
    width: "100%",
    paddingVertical: 8,
  },
  buttonLabel: {
    fontSize: 18,
  },
});
