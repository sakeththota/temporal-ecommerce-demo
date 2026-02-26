"use client";

import { useState, useEffect, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  createBooking,
  getBookingProgress,
  crashServer,
  getHotels,
} from "@/lib/api";
import type { BookingProgress, Hotel } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  MapPin,
  ArrowLeft,
  Check,
  Loader2,
  AlertTriangle,
  CircleDot,
  Circle,
  CheckCircle2,
  XCircle,
  Zap,
  User,
  Mail,
  CalendarDays,
} from "lucide-react";

function BookingPageContent() {
  const searchParams = useSearchParams();
  const hotelId = searchParams.get("hotel");

  const [step, setStep] = useState<
    "select" | "checkout" | "processing" | "completed"
  >("select");
  const [hotels, setHotels] = useState<Hotel[]>([]);
  const [selectedHotel, setSelectedHotel] = useState<Hotel | null>(null);
  const [guestName, setGuestName] = useState("");
  const [guestEmail, setGuestEmail] = useState("");
  const [checkIn, setCheckIn] = useState("");
  const [checkOut, setCheckOut] = useState("");
  const [workflowId, setWorkflowId] = useState("");
  const [progress, setProgress] = useState<BookingProgress | null>(null);
  const [loading, setLoading] = useState(false);
  const [crashing, setCrashing] = useState(false);

  useEffect(() => {
    getHotels()
      .then((hotels) => {
        setHotels(hotels);
        if (hotelId) {
          const hotel = hotels.find((h) => h.id === hotelId);
          if (hotel) {
            setSelectedHotel(hotel);
            setStep("checkout");
          }
        }
      })
      .catch(console.error);
  }, [hotelId]);

  useEffect(() => {
    if (!workflowId || step !== "processing") return;

    const interval = setInterval(async () => {
      try {
        const p = await getBookingProgress(workflowId);
        setProgress(p);
        if (p.status === "completed") {
          setStep("completed");
          clearInterval(interval);
        } else if (p.status === "failed") {
          clearInterval(interval);
        }
      } catch (err) {
        console.error("polling error:", err);
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [workflowId, step]);

  const handleSelectHotel = (hotel: Hotel) => {
    setSelectedHotel(hotel);
    setStep("checkout");
  };

  const handleBook = async () => {
    if (!selectedHotel || !guestName || !guestEmail || !checkIn || !checkOut) {
      return;
    }

    const nights = Math.ceil(
      (new Date(checkOut).getTime() - new Date(checkIn).getTime()) /
        (1000 * 60 * 60 * 24)
    );
    if (nights <= 0) return;

    setLoading(true);
    try {
      const result = await createBooking({
        guest_name: guestName,
        guest_email: guestEmail,
        items: [
          {
            hotel_id: selectedHotel.id,
            check_in: checkIn,
            check_out: checkOut,
            nights,
            price_per_night: selectedHotel.price_per_night,
            subtotal: nights * selectedHotel.price_per_night,
          },
        ],
      });
      setWorkflowId(result.workflow_id);
      setStep("processing");
    } catch (err) {
      console.error("booking failed:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleCrash = async () => {
    setCrashing(true);
    try {
      await crashServer();
    } catch (err) {
      console.error("crash failed:", err);
    }
  };

  // Step 1: Hotel Selection
  if (step === "select") {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Select a Hotel</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Choose from our available properties to start your booking.
          </p>
        </div>

        {hotels.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {hotels.map((hotel) => (
              <Card
                key={hotel.id}
                className="group cursor-pointer overflow-hidden transition-all hover:shadow-md hover:ring-1 hover:ring-primary/20"
                onClick={() => handleSelectHotel(hotel)}
              >
                {hotel.image_url && (
                  <div className="aspect-[4/3] overflow-hidden">
                    <img
                      src={hotel.image_url}
                      alt={hotel.name}
                      className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                    />
                  </div>
                )}
                <div className="p-4">
                  <h3 className="font-semibold text-foreground">
                    {hotel.name}
                  </h3>
                  <div className="mt-1 flex items-center gap-1 text-sm text-muted-foreground">
                    <MapPin className="h-3 w-3" />
                    {hotel.location}
                  </div>
                  <div className="mt-3 text-lg font-bold text-foreground">
                    ${hotel.price_per_night}
                    <span className="text-sm font-normal text-muted-foreground">
                      {" "}
                      / night
                    </span>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>
    );
  }

  // Step 2: Checkout
  if (step === "checkout" && selectedHotel) {
    const nights =
      checkIn && checkOut
        ? Math.ceil(
            (new Date(checkOut).getTime() - new Date(checkIn).getTime()) /
              (1000 * 60 * 60 * 24)
          )
        : 0;
    const total = nights * selectedHotel.price_per_night;

    return (
      <div className="space-y-6">
        <button
          onClick={() => setStep("select")}
          className="flex items-center gap-1 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to hotels
        </button>

        <div>
          <h1 className="text-2xl font-bold tracking-tight">
            Complete Your Booking
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Fill in your details to finalize the reservation.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-5">
          {/* Form - left side */}
          <div className="space-y-5 lg:col-span-3">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Guest Information</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium text-foreground">
                    Full Name
                  </label>
                  <div className="relative">
                    <User className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      type="text"
                      value={guestName}
                      onChange={(e) => setGuestName(e.target.value)}
                      placeholder="John Doe"
                      className="pl-9"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium text-foreground">
                    Email Address
                  </label>
                  <div className="relative">
                    <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      type="email"
                      value={guestEmail}
                      onChange={(e) => setGuestEmail(e.target.value)}
                      placeholder="john@example.com"
                      className="pl-9"
                    />
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Stay Dates</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-foreground">
                      Check-in
                    </label>
                    <div className="relative">
                      <CalendarDays className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        type="date"
                        value={checkIn}
                        onChange={(e) => setCheckIn(e.target.value)}
                        className="pl-9"
                      />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-sm font-medium text-foreground">
                      Check-out
                    </label>
                    <div className="relative">
                      <CalendarDays className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        type="date"
                        value={checkOut}
                        onChange={(e) => setCheckOut(e.target.value)}
                        className="pl-9"
                      />
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Summary - right side */}
          <div className="lg:col-span-2">
            <Card className="sticky top-20">
              <div className="overflow-hidden rounded-t-lg">
                {selectedHotel.image_url && (
                  <img
                    src={selectedHotel.image_url}
                    alt={selectedHotel.name}
                    className="h-40 w-full object-cover"
                  />
                )}
              </div>
              <CardContent className="space-y-4 pt-4">
                <div>
                  <h3 className="font-semibold text-foreground">
                    {selectedHotel.name}
                  </h3>
                  <div className="mt-1 flex items-center gap-1 text-sm text-muted-foreground">
                    <MapPin className="h-3 w-3" />
                    {selectedHotel.location}
                  </div>
                </div>

                <div className="space-y-2 border-t pt-4">
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Rate per night</span>
                    <span className="font-medium">
                      ${selectedHotel.price_per_night}
                    </span>
                  </div>
                  {nights > 0 && (
                    <>
                      <div className="flex justify-between text-sm">
                        <span className="text-muted-foreground">
                          {nights} night{nights > 1 ? "s" : ""}
                        </span>
                        <span className="font-medium">${total.toFixed(2)}</span>
                      </div>
                      <div className="flex justify-between border-t pt-2 text-base font-bold">
                        <span>Total</span>
                        <span>${total.toFixed(2)}</span>
                      </div>
                    </>
                  )}
                </div>

                <Button
                  onClick={handleBook}
                  disabled={
                    loading ||
                    !guestName ||
                    !guestEmail ||
                    !checkIn ||
                    !checkOut ||
                    nights <= 0
                  }
                  className="w-full"
                  size="lg"
                >
                  {loading ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Processing...
                    </>
                  ) : (
                    "Confirm Booking"
                  )}
                </Button>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    );
  }

  // Step 3: Processing
  if (step === "processing") {
    const steps = [
      { key: "validating_availability", label: "Validating Availability" },
      { key: "processing_payment", label: "Processing Payment" },
      { key: "reserving_booking", label: "Reserving Booking" },
      { key: "sending_confirmation", label: "Sending Confirmation" },
    ];

    const currentStepIndex = steps.findIndex(
      (s) => s.key === progress?.current_step
    );

    return (
      <div className="mx-auto max-w-xl space-y-6">
        <div className="text-center">
          <h1 className="text-2xl font-bold tracking-tight">
            Booking in Progress
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Your reservation is being processed by a durable Temporal workflow.
          </p>
        </div>

        <Card>
          <CardContent className="space-y-6 pt-6">
            {/* Workflow ID */}
            <div className="flex items-center justify-between rounded-lg bg-muted px-3 py-2">
              <span className="text-xs text-muted-foreground">Workflow ID</span>
              <code className="text-xs font-medium">{workflowId}</code>
            </div>

            {/* Steps visualization */}
            <div className="space-y-3">
              {steps.map((s, i) => {
                const isCompleted = currentStepIndex > i || progress?.status === "completed";
                const isCurrent = s.key === progress?.current_step && progress?.status !== "completed";
                const isFailed =
                  s.key === progress?.current_step &&
                  progress?.status === "failed";

                return (
                  <div key={s.key} className="flex items-center gap-3">
                    <div className="flex-shrink-0">
                      {isFailed ? (
                        <XCircle className="h-5 w-5 text-destructive" />
                      ) : isCompleted ? (
                        <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                      ) : isCurrent ? (
                        <CircleDot className="h-5 w-5 animate-pulse text-primary" />
                      ) : (
                        <Circle className="h-5 w-5 text-muted-foreground/30" />
                      )}
                    </div>
                    <span
                      className={`text-sm ${
                        isCompleted
                          ? "font-medium text-emerald-600"
                          : isCurrent
                          ? "font-medium text-foreground"
                          : isFailed
                          ? "font-medium text-destructive"
                          : "text-muted-foreground"
                      }`}
                    >
                      {s.label}
                    </span>
                    {isCurrent && !isFailed && (
                      <Loader2 className="ml-auto h-4 w-4 animate-spin text-primary" />
                    )}
                  </div>
                );
              })}
            </div>

            {progress?.error && (
              <div className="flex items-start gap-2 rounded-lg bg-red-50 p-3 text-sm text-red-700">
                <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0" />
                <span>{progress.error}</span>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Crash demo button */}
        {progress?.status === "processing" && (
          <Card className="border-dashed border-amber-300 bg-amber-50/50">
            <CardContent className="flex items-center justify-between pt-6">
              <div className="space-y-1">
                <p className="text-sm font-medium text-amber-800">
                  Durability Demo
                </p>
                <p className="text-xs text-amber-600">
                  Crash the server to see Temporal recover the workflow.
                </p>
              </div>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleCrash}
                disabled={crashing}
              >
                <Zap className="h-3.5 w-3.5" />
                {crashing ? "Crashing..." : "Crash Server"}
              </Button>
            </CardContent>
          </Card>
        )}

        {progress?.status === "failed" && (
          <div className="text-center">
            <Button
              variant="outline"
              onClick={() => {
                setStep("checkout");
                setWorkflowId("");
                setProgress(null);
              }}
            >
              Try Again
            </Button>
          </div>
        )}
      </div>
    );
  }

  // Step 4: Completed
  if (step === "completed") {
    return (
      <div className="mx-auto max-w-lg space-y-6">
        <Card className="overflow-hidden">
          <div className="bg-gradient-to-br from-emerald-500 to-emerald-600 px-6 py-8 text-center text-white">
            <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-white/20">
              <Check className="h-6 w-6" />
            </div>
            <h1 className="text-2xl font-bold">Booking Confirmed!</h1>
            <p className="mt-1 text-emerald-100">
              Your reservation has been successfully processed.
            </p>
          </div>
          <CardContent className="space-y-4 pt-6">
            {selectedHotel && (
              <div className="flex items-center gap-3">
                {selectedHotel.image_url && (
                  <img
                    src={selectedHotel.image_url}
                    alt={selectedHotel.name}
                    className="h-14 w-14 rounded-lg object-cover"
                  />
                )}
                <div>
                  <p className="font-semibold">{selectedHotel.name}</p>
                  <div className="flex items-center gap-1 text-sm text-muted-foreground">
                    <MapPin className="h-3 w-3" />
                    {selectedHotel.location}
                  </div>
                </div>
              </div>
            )}

            {progress?.total_amount && (
              <div className="flex justify-between rounded-lg bg-muted px-4 py-3">
                <span className="font-medium">Total Charged</span>
                <span className="text-lg font-bold">
                  ${progress.total_amount}
                </span>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => {
                  setStep("select");
                  setWorkflowId("");
                  setProgress(null);
                  setSelectedHotel(null);
                }}
              >
                Book Another
              </Button>
              <Button asChild className="flex-1">
                <Link href="/">Search Hotels</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return null;
}

export default function BookingPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <BookingPageContent />
    </Suspense>
  );
}
